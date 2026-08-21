package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New builds the process-wide slog JSON logger. Production (and any non-debug
// level) drops debug records. Callers must not use fmt.Println.
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return a
			}
			return a
		},
	})
	log := slog.New(h).With("service", "gojira")
	slog.SetDefault(log)
	return log
}
