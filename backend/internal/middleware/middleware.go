package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"gojira/internal/auth"
	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			var b [16]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), platform.CtxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "panic", rec, "path", r.URL.Path, "request_id", platform.RequestIDFrom(r))
					platform.WriteError(w, r, domain.Internal(nil))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID,Location")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func JWT(a *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
				platform.WriteError(w, r, domain.Unauthorized("缺少访问令牌"))
				return
			}
			raw := strings.TrimSpace(h[7:])
			c, err := a.Parse(raw)
			if err != nil || c.Kind != "access" {
				platform.WriteError(w, r, domain.Unauthorized("访问令牌无效"))
				return
			}
			u, err := a.LoadUser(r.Context(), c.UserID)
			if err != nil {
				platform.WriteError(w, r, domain.Unauthorized("用户不存在或已停用"))
				return
			}
			p := &platform.UserPrincipal{
				ID: u.ID, Username: u.Username, Email: u.Email,
				DisplayName: u.DisplayName, Role: c.Role,
			}
			ctx := context.WithValue(r.Context(), platform.CtxUser, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireProject(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			param := strings.TrimSpace(chi.URLParam(r, "id"))
			id, err := platform.ParseID(param)
			if err != nil {
				if qerr := db.GetContext(r.Context(), &id, `SELECT id FROM projects WHERE key=$1`, strings.ToUpper(param)); qerr != nil {
					platform.WriteError(w, r, domain.NotFound("项目不存在"))
					return
				}
			}
			attachProject(w, r, next, db, id)
		})
	}
}

func RequireIssueProject(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := platform.ParseID(chi.URLParam(r, "id"))
			if err != nil {
				platform.WriteError(w, r, err)
				return
			}
			var pid int64
			if err := db.GetContext(r.Context(), &pid, `SELECT project_id FROM issues WHERE id=$1`, id); err != nil {
				platform.WriteError(w, r, domain.NotFound("事项不存在"))
				return
			}
			attachProject(w, r, next, db, pid)
		})
	}
}

func RequireSprintProject(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := platform.ParseID(chi.URLParam(r, "id"))
			if err != nil {
				platform.WriteError(w, r, err)
				return
			}
			var pid int64
			if err := db.GetContext(r.Context(), &pid, `SELECT project_id FROM sprints WHERE id=$1`, id); err != nil {
				platform.WriteError(w, r, domain.NotFound("迭代不存在"))
				return
			}
			attachProject(w, r, next, db, pid)
		})
	}
}

func attachProject(w http.ResponseWriter, r *http.Request, next http.Handler, db *sqlx.DB, projectID int64) {
	u := platform.UserFrom(r)
	if u == nil {
		platform.WriteError(w, r, domain.Unauthorized("未登录"))
		return
	}
	var role string
	err := db.GetContext(r.Context(), &role, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2`, projectID, u.ID)
	if err != nil {
		if u.Role == domain.RoleAdmin {
			role = domain.RoleAdmin
		} else {
			platform.WriteError(w, r, domain.Forbidden("不是该项目成员", nil))
			return
		}
	}
	if role == domain.RoleViewer && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
		platform.WriteError(w, r, domain.Forbidden("访客仅可只读", nil))
		return
	}
	ctx := context.WithValue(r.Context(), platform.CtxProjectID, projectID)
	ctx = context.WithValue(ctx, platform.CtxProjectRole, role)
	next.ServeHTTP(w, r.WithContext(ctx))
}
