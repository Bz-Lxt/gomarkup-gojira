package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// OpenPostgres connects with Asia/Shanghai session timezone and retries.
func OpenPostgres(dsn string, log *slog.Logger) (*sqlx.DB, error) {
	var db *sqlx.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			break
		}
		log.Warn("postgres connect retry", "attempt", i+1, "err", err)
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if _, err := db.Exec("SET TIME ZONE 'Asia/Shanghai'"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set timezone: %w", err)
	}
	return db, nil
}

func InsertAudit(ctx context.Context, ext sqlx.ExtContext, actorID *int64, action, entityType, entityID string, detail any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		raw = []byte("{}")
	}
	_, err = ext.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		actorID, action, entityType, entityID, raw, Now())
	return err
}
