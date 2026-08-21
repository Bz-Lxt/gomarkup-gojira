package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is loaded exclusively from environment variables.
type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPass       string
	SMTPFrom       string
	SMTPTLS        bool
	LogLevel       string
	WorkflowPath   string
	MigrationsPath string
	AppEnv         string
}

// Load reads process environment. Missing required values return an error.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:       envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		SMTPHost:       envOr("SMTP_HOST", "mailpit"),
		SMTPPort:       envInt("SMTP_PORT", 1025),
		SMTPUser:       os.Getenv("SMTP_USER"),
		SMTPPass:       os.Getenv("SMTP_PASS"),
		SMTPFrom:       envOr("SMTP_FROM", "gojira@local.dev"),
		SMTPTLS:        envBool("SMTP_TLS", false),
		LogLevel:       envOr("LOG_LEVEL", "info"),
		WorkflowPath:   envOr("WORKFLOW_PATH", ""),
		MigrationsPath: envOr("MIGRATIONS_PATH", "migrations"),
		AppEnv:         envOr("APP_ENV", "development"),
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if cfg.WorkflowPath == "" {
		cfg.WorkflowPath = ResolveWorkflowPath()
	}
	return cfg, nil
}

// ResolveWorkflowPath prefers the container mount, then repo-relative files.
func ResolveWorkflowPath() string {
	candidates := []string{
		os.Getenv("WORKFLOW_PATH"),
		"/app/config/workflow.yaml",
		"config/workflow.yaml",
		"../config/workflow.yaml",
		"../../config/workflow.yaml",
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return "/app/config/workflow.yaml"
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
