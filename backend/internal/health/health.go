package health

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"gojira/internal/config"
	"gojira/internal/platform"

	"github.com/jmoiron/sqlx"
)

type Handler struct {
	DB  *sqlx.DB
	Cfg *config.Config
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dbOK := true
	if err := h.DB.Ping(); err != nil {
		dbOK = false
	}
	smtpOK := probeSMTP(h.Cfg.SMTPHost, h.Cfg.SMTPPort)
	status := http.StatusOK
	if !dbOK {
		status = http.StatusServiceUnavailable
	}
	platform.WriteJSON(w, status, map[string]any{
		"status":   map[bool]string{true: "ok", false: "degraded"}[dbOK && smtpOK],
		"db":       dbOK,
		"smtp":     smtpOK,
		"time":     platform.Now().Format(time.RFC3339),
		"timezone": "Asia/Shanghai",
	})
}

func probeSMTP(host string, port int) bool {
	if host == "" {
		return false
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
