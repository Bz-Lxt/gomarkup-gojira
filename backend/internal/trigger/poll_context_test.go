package trigger_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gojira/internal/domain"
	"gojira/internal/trigger"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestPollDeliversEveryWebhookInBatch(t *testing.T) {
	var calls atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(receiver.Close)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	xdb := sqlx.NewDb(db, "postgres")
	t.Cleanup(func() { _ = xdb.Close() })

	now := time.Now()
	firstPayload, err := json.Marshal(domain.StatusChangedPayload{
		EventID: "event-first", IssueID: 101, ProjectID: 7, IssueKey: "GJ-101",
		FromStatus: "TESTING", ToStatus: "DONE", ToColumn: "DONE",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondPayload, err := json.Marshal(domain.StatusChangedPayload{
		EventID: "event-second", IssueID: 102, ProjectID: 7, IssueKey: "GJ-102",
		FromStatus: "TESTING", ToStatus: "DONE", ToColumn: "DONE",
	})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT \\* FROM outbox_events").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "payload", "status", "retry_count",
			"error_class", "error_msg", "next_attempt_at", "created_at", "updated_at",
		}).
			AddRow(1, "event-first", domain.EventIssueStatusChanged, firstPayload, domain.OutboxPending, 0, nil, nil, nil, now, now).
			AddRow(2, "event-second", domain.EventIssueStatusChanged, secondPayload, domain.OutboxPending, 0, nil, nil, nil, now, now))

	actions, err := json.Marshal([]trigger.Action{{Type: domain.ActionWebhook, URL: receiver.URL}})
	if err != nil {
		t.Fatal(err)
	}
	triggerRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at",
		}).AddRow(11, 7, "notify release", domain.EventIssueStatusChanged, []byte(`{}`), actions, true, now)
	}
	for i := range 2 {
		query := mock.ExpectQuery("SELECT \\* FROM triggers")
		if i == 1 {
			query.WillDelayFor(100 * time.Millisecond)
		}
		query.WillReturnRows(triggerRows())
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM trigger_executions").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectExec("INSERT INTO trigger_executions").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	}

	worker := &trigger.Worker{
		DB:  xdb,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := worker.Poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("webhook calls = %d, want 2 for two pending events", got)
	}
}
