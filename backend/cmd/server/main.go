package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gojira/internal/config"
	"gojira/internal/logger"
	"gojira/internal/migrate"
	"gojira/internal/platform"
	"gojira/internal/router"
	"gojira/internal/seed"
	"gojira/internal/stats"
	"gojira/internal/trigger"
	"gojira/internal/workflow"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Logger is not ready; write to stderr via slog after a fallback logger.
		log := logger.New("error")
		log.Error("load config", "err", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)
	log.Info("starting gojira", "tz", "Asia/Shanghai", "now", platform.Now())

	db, err := platform.OpenPostgres(cfg.DatabaseURL, log)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := migrate.Up(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	engine, err := workflow.Load(cfg.WorkflowPath)
	if err != nil {
		log.Error("workflow load/validate failed", "path", cfg.WorkflowPath, "err", err)
		os.Exit(1)
	}
	log.Info("workflow loaded", "path", cfg.WorkflowPath, "workflows", len(engine.File.Workflows))

	if err := seed.Run(db, log); err != nil {
		log.Error("seed", "err", err)
		os.Exit(1)
	}

	h := router.New(router.Deps{DB: db, Cfg: cfg, Log: log, Engine: engine})
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker := &trigger.Worker{DB: db, Cfg: cfg, Log: log, Engine: engine}
	go worker.Start(ctx)

	st := &stats.Service{DB: db, Log: log}
	go st.SnapshotLoop(ctx)

	go func() {
		log.Info("http listen", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
