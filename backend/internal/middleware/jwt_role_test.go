package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"gojira/internal/auth"
	"gojira/internal/domain"
	"gojira/internal/middleware"
	"gojira/internal/project"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestJWTUsesCurrentUserRole(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })
	db := sqlx.NewDb(rawDB, "sqlmock")

	const secret = "test-secret-long-enough"
	authService := &auth.Service{DB: db, Secret: []byte(secret)}
	token, err := authService.Sign(&domain.User{
		ID:       7,
		Username: "former-admin",
		Role:     domain.RoleAdmin,
	}, "access", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, time.August, 24, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM users WHERE id=$1 AND is_active=TRUE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "password_hash", "display_name", "role", "is_active", "created_at", "updated_at",
		}).AddRow(7, "former-admin", "former-admin@example.test", "unused", "Former Admin", domain.RoleViewer, true, now, now))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO projects`).
		WithArgs("SEC", "Security", "", int64(7), false, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "key", "name", "description", "owner_id", "enforce_dependency_block", "workflow_config", "created_at", "updated_at",
		}).AddRow(42, "SEC", "Security", "", 7, false, "default", now, now))
	mock.ExpectExec(`INSERT INTO project_members`).
		WithArgs(int64(42), int64(7), domain.RoleAdmin, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WithArgs(int64(7), "project.create", "project", "SEC-42", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	handler := middleware.JWT(authService)(http.HandlerFunc((&project.Service{DB: db}).Create))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"key":"SEC","name":"Security"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("role demotion must invalidate old write authority: got status %d, body %s", rec.Code, rec.Body.String())
	}
	var body domain.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != domain.CodeForbidden {
		t.Fatalf("got error code %q, want %q", body.Code, domain.CodeForbidden)
	}
}
