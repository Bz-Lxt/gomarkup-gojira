package issue_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gojira/internal/domain"
	"gojira/internal/issue"
	"gojira/internal/platform"
	"gojira/internal/workflow"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func TestTransitionStopsWhenDependencyLookupFails(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	engine, err := workflow.Parse([]byte(`
version: 1
roles: [DEV]
columns:
  - {id: TODO, label: Todo}
  - {id: IN_PROGRESS, label: In progress}
workflows:
  task:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: TODO}
      IN_PROGRESS: {column: IN_PROGRESS, terminal: true}
    transitions:
      - {from: TODO, to: IN_PROGRESS, allowed_roles: [DEV]}
`))
	if err != nil {
		t.Fatal(err)
	}

	createdAt := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	issueRows := func(status string, version int64) *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id", "project_id", "seq_no", "issue_type", "title", "description",
			"status", "priority", "reporter_id", "board_rank", "version",
			"created_at", "updated_at", "issue_key", "project_key",
		}).AddRow(
			int64(82), int64(1), int64(82), domain.TypeTask, "Blocked task", "",
			status, "MEDIUM", int64(9), float64(1000), version,
			createdAt, createdAt, "GJ-82", "GJ",
		)
	}
	emptyLabels := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"label"})
	}

	mock.ExpectQuery("FROM issues i JOIN projects p").
		WithArgs(int64(82)).
		WillReturnRows(issueRows("TODO", 1))
	mock.ExpectQuery("SELECT label FROM issue_labels").
		WithArgs(int64(82)).
		WillReturnRows(emptyLabels())
	mock.ExpectQuery("FROM projects WHERE id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "key", "name", "description", "owner_id", "enforce_dependency_block",
			"workflow_config", "created_at", "updated_at",
		}).AddRow(
			int64(1), "GJ", "GoJira", "", int64(1), true,
			"default", createdAt, createdAt,
		))
	mock.ExpectQuery("FROM issue_dependencies d").
		WithArgs(int64(82)).
		WillReturnError(errors.New("pq: canceling statement due to statement timeout"))

	mock.ExpectQuery("FROM issue_status_history").
		WillReturnRows(sqlmock.NewRows([]string{"changed_at"}).AddRow(createdAt))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues SET status").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO issue_status_history").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO outbox_events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("FROM issues i JOIN projects p").
		WithArgs(int64(82)).
		WillReturnRows(issueRows("IN_PROGRESS", 2))
	mock.ExpectQuery("SELECT label FROM issue_labels").
		WithArgs(int64(82)).
		WillReturnRows(emptyLabels())

	svc := &issue.Service{DB: sqlx.NewDb(db, "postgres"), Engine: engine}
	router := chi.NewRouter()
	router.Post("/issues/{id}/transition", svc.Transition)

	req := httptest.NewRequest(http.MethodPost, "/issues/82/transition", strings.NewReader(`{"to":"IN_PROGRESS","version":1}`))
	ctx := context.WithValue(req.Context(), platform.CtxProjectRole, domain.RoleDev)
	ctx = context.WithValue(ctx, platform.CtxUser, &platform.UserPrincipal{ID: 9, Role: domain.RoleDev})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rr.Body.String())
	}
	if rr.Code != http.StatusInternalServerError || body.Code != domain.CodeInternal {
		t.Fatalf("dependency lookup failure must stop the transition with 500/INTERNAL; got %d/%s: %s", rr.Code, body.Code, rr.Body.String())
	}
}
