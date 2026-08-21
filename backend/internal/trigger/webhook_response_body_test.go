package trigger_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gojira/internal/domain"
	"gojira/internal/trigger"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type observedBody struct {
	reader *strings.Reader
	closed atomic.Bool
}

func (b *observedBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *observedBody) Close() error {
	b.closed.Store(true)
	return nil
}

func TestWorkerClosesWebhookResponseBodyOnServerError(t *testing.T) {
	rawDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	bodyText := "temporarily unavailable"
	body := &observedBody{reader: strings.NewReader(bodyText)}
	t.Cleanup(func() { _ = body.Close() })

	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	var calls atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			Status:        "503 Service Unavailable",
			StatusCode:    http.StatusServiceUnavailable,
			Header:        make(http.Header),
			Body:          body,
			ContentLength: int64(len(bodyText)),
			Request:       req,
		}, nil
	})

	now := time.Now()
	payload, err := json.Marshal(domain.StatusChangedPayload{
		EventID: "issue.status_changed-response-cleanup",
		IssueID: 27, ProjectID: 7, IssueKey: "GJ-27",
		ToStatus: "DONE", ToColumn: "DONE",
	})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("outbox_events").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "payload", "status", "retry_count",
			"error_class", "error_msg", "next_attempt_at", "created_at", "updated_at",
		}).AddRow(27, "issue.status_changed-response-cleanup", domain.EventIssueStatusChanged,
			payload, domain.OutboxPending, 0, nil, nil, nil, now, now))
	mock.ExpectQuery("FROM triggers").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at",
		}).AddRow(9, 7, "partner callback", domain.EventIssueStatusChanged, []byte(`{}`),
			[]byte(`[{"type":"WEBHOOK","url":"http://partner.invalid/hook"}]`), true, now))
	mock.ExpectQuery("COUNT\\(\\*\\) FROM trigger_executions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE outbox_events SET retry_count").
		WillReturnResult(sqlmock.NewResult(0, 1))

	worker := &trigger.Worker{
		DB:  sqlx.NewDb(rawDB, "postgres"),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := worker.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("webhook calls = %d, want 1", calls.Load())
	}
	if !body.closed.Load() {
		t.Fatal("webhook response was not released after the worker finished the delivery attempt")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
