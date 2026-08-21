package trigger

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gojira/internal/config"
	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestClassifyErrorTable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"timeout", errors.New("i/o timeout"), domain.ErrorTransient},
		{"conn refused", errors.New("connection refused"), domain.ErrorTransient},
		{"smtp 421", errors.New("421 service not available"), domain.ErrorTransient},
		{"smtp 450", errors.New("450 mailbox busy"), domain.ErrorTransient},
		{"auth 535", errors.New("535 authentication failed"), domain.ErrorPermanent},
		{"invalid addr", errors.New("validation: invalid recipient"), domain.ErrorPermanent},
		{"classified permanent", &ClassifiedError{Class: domain.ErrorPermanent, Msg: "no recipients"}, domain.ErrorPermanent},
		{"classified transient", &ClassifiedError{Class: domain.ErrorTransient, Msg: "try later"}, domain.ErrorTransient},
		{"net timeout", timeoutErr{}, domain.ErrorTransient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "deadline exceeded" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		class string
		n     int
		want  bool
	}{
		{domain.ErrorTransient, 0, true},
		{domain.ErrorTransient, 1, true},
		{domain.ErrorTransient, 2, true},
		{domain.ErrorTransient, 3, false},
		{domain.ErrorTransient, 4, false},
		{domain.ErrorPermanent, 0, false},
		{domain.ErrorPermanent, 1, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		if got := ShouldRetry(tt.class, tt.n); got != tt.want {
			t.Fatalf("ShouldRetry(%s,%d)=%v want %v", tt.class, tt.n, got, tt.want)
		}
	}
}

func TestNextBackoffExponential(t *testing.T) {
	if NextBackoff(0) != 2*time.Second {
		t.Fatal(NextBackoff(0))
	}
	if NextBackoff(1) != 4*time.Second {
		t.Fatal(NextBackoff(1))
	}
	if NextBackoff(2) != 8*time.Second {
		t.Fatal(NextBackoff(2))
	}
}

func TestExecutionKeyIdempotent(t *testing.T) {
	store := map[string]struct{}{}
	record := func(eventID string, triggerID int64, action string) bool {
		k := ExecutionKey(eventID, triggerID, action)
		if _, ok := store[k]; ok {
			return false
		}
		store[k] = struct{}{}
		return true
	}
	if !record("evt-1", 9, domain.ActionSendEmail) {
		t.Fatal("first insert")
	}
	if record("evt-1", 9, domain.ActionSendEmail) {
		t.Fatal("duplicate event_id must be rejected")
	}
	if !record("evt-1", 9, domain.ActionInAppNotify) {
		t.Fatal("different action is a new row")
	}
	if !record("evt-2", 9, domain.ActionSendEmail) {
		t.Fatal("different event is a new row")
	}
}

func TestMatchCondition(t *testing.T) {
	p := domain.StatusChangedPayload{ToColumn: "DONE", ToStatus: "DONE", IssueType: "TASK", FromStatus: "TESTING"}
	if !MatchCondition(nil, p) {
		t.Fatal("empty matches")
	}
	if !MatchCondition(map[string]any{"to_column": "DONE"}, p) {
		t.Fatal("column DONE")
	}
	if MatchCondition(map[string]any{"to_column": "TODO"}, p) {
		t.Fatal("should reject TODO column")
	}
	if MatchCondition(map[string]any{"issue_type": "BUG"}, p) {
		t.Fatal("type mismatch")
	}
}

type fakeMailer struct {
	err  error
	sent []string
}

func (f *fakeMailer) Send(to []string, subject, body string) error {
	f.sent = append(f.sent, subject)
	return f.err
}

func TestSMTPMailerEmptyRecipients(t *testing.T) {
	err := SMTPMailer{From: "a@b.c", Host: "127.0.0.1", Port: 9}.Send(nil, "s", "b")
	if ClassifyError(err) != domain.ErrorPermanent {
		t.Fatalf("%v", err)
	}
	if err == nil || err.Error() == "" {
		t.Fatal("classified error string")
	}
}

func TestRunActionTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	fail5 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(fail5.Close)
	unauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(unauth.Close)

	mail := &fakeMailer{}
	w := &Worker{Mailer: mail, Log: slog.Default()}
	p := domain.StatusChangedPayload{IssueKey: "GJ-1", Title: "t", ToStatus: "DONE", ToColumn: "DONE", ProjectID: 1}

	tests := []struct {
		name    string
		act     Action
		wantErr bool
		class   string
	}{
		{"email to explicit", Action{Type: domain.ActionSendEmail, To: "pm@local"}, false, ""},
		{"email mailer fail transient", Action{Type: domain.ActionSendEmail, To: "pm@local"}, true, domain.ErrorTransient},
		{"in-app", Action{Type: domain.ActionInAppNotify, Body: "hi"}, false, ""},
		{"webhook ok", Action{Type: domain.ActionWebhook, URL: srv.URL}, false, ""},
		{"webhook 5xx", Action{Type: domain.ActionWebhook, URL: fail5.URL}, true, domain.ErrorTransient},
		{"webhook 401", Action{Type: domain.ActionWebhook, URL: unauth.URL}, true, domain.ErrorPermanent},
		{"webhook missing", Action{Type: domain.ActionWebhook}, true, domain.ErrorPermanent},
		{"comment empty", Action{Type: domain.ActionAddComment}, true, domain.ErrorPermanent},
		{"assign missing", Action{Type: domain.ActionAutoAssign}, true, domain.ErrorPermanent},
		{"unknown", Action{Type: "NOPE"}, true, domain.ErrorPermanent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "email mailer fail transient" {
				w.Mailer = &fakeMailer{err: errors.New("i/o timeout")}
			} else {
				w.Mailer = mail
			}
			err := w.runAction(context.Background(), tt.act, p)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr && tt.class != "" && ClassifyError(err) != tt.class {
				t.Fatalf("class %s want %s (%v)", ClassifyError(err), tt.class, err)
			}
		})
	}
}

func TestPostWebhookBadURL(t *testing.T) {
	err := postWebhook(":", domain.StatusChangedPayload{})
	if ClassifyError(err) != domain.ErrorPermanent {
		t.Fatalf("%v", err)
	}
}

func TestNextBackoffClamp(t *testing.T) {
	if NextBackoff(-1) != 2*time.Second {
		t.Fatal(NextBackoff(-1))
	}
	if NextBackoff(10) < NextBackoff(4) {
		t.Fatal("should clamp")
	}
}

func TestMatchConditionFromStatus(t *testing.T) {
	p := domain.StatusChangedPayload{FromStatus: "TESTING", ToStatus: "DONE", ToColumn: "DONE", IssueType: "TASK"}
	if !MatchCondition(map[string]any{"from_status": "TESTING"}, p) {
		t.Fatal()
	}
	if MatchCondition(map[string]any{"from_status": "TODO", "to_status": "DONE"}, p) {
		t.Fatal()
	}
}

func TestMaxRetriesIsThree(t *testing.T) {
	if MaxRetries != 3 {
		t.Fatalf("MaxRetries=%d", MaxRetries)
	}
}

func TestWorkerPollAndDeliver(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	x := sqlx.NewDb(db, "postgres")
	mail := &fakeMailer{}
	w := &Worker{DB: x, Mailer: mail, Log: slog.Default()}

	now := time.Now()
	// non status-changed → mark done
	mock.ExpectQuery("outbox_events").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "payload", "status", "retry_count",
			"error_class", "error_msg", "next_attempt_at", "created_at", "updated_at",
		}).AddRow(1, "e-sprint", domain.EventSprintStarted, []byte(`{}`), "pending", 0, nil, nil, nil, now, now))
	mock.ExpectExec("UPDATE outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(domain.StatusChangedPayload{
		EventID: "e-done", IssueID: 9, ProjectID: 1, IssueKey: "GJ-1",
		Title: "t", ToStatus: "DONE", ToColumn: "DONE",
	})
	mock.ExpectQuery("outbox_events").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "payload", "status", "retry_count",
			"error_class", "error_msg", "next_attempt_at", "created_at", "updated_at",
		}).AddRow(2, "e-done", domain.EventIssueStatusChanged, payload, "pending", 0, nil, nil, nil, now, now))
	mock.ExpectQuery("FROM triggers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at"}).
			AddRow(4, 1, "pm mail", domain.EventIssueStatusChanged, []byte(`{"to_column":"DONE"}`),
				[]byte(`[{"type":"SEND_EMAIL","to":"pm@local"}]`), true, now))
	mock.ExpectQuery("COUNT\\(\\*\\) FROM trigger_executions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT user_id FROM project_members").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(2))
	mock.ExpectExec("INSERT INTO notifications").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO trigger_executions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) == 0 {
		t.Fatal("expected email")
	}

	// invalid payload → dead
	ev := &domain.OutboxEvent{ID: 3, EventID: "bad", EventType: domain.EventIssueStatusChanged, Payload: []byte(`{`)}
	mock.ExpectExec("UPDATE outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	w.deliver(context.Background(), ev)

	// idempotent execOnce
	mock.ExpectQuery("trigger_executions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	ok := w.execOnce(context.Background(), &domain.OutboxEvent{EventID: "e-done"}, domain.Trigger{ID: 4}, Action{Type: domain.ActionSendEmail}, domain.StatusChangedPayload{})
	if !ok {
		t.Fatal("existing execution is success")
	}

	api := &API{DB: x}
	mock.ExpectQuery("FROM triggers WHERE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at"}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/triggers", nil)
	req = req.WithContext(context.WithValue(req.Context(), platform.CtxProjectID, int64(1)))
	api.List(rr, req)
	if rr.Code != 200 {
		t.Fatalf("list %d", rr.Code)
	}

	mock.ExpectQuery("COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("trigger_executions e").
		WillReturnRows(sqlmock.NewRows([]string{"id", "trigger_id", "event_id", "action_type", "status", "error_class", "error_msg", "retry_count", "duration_ms", "created_at"}))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/exec?page=1&per_page=20", nil)
	req = req.WithContext(context.WithValue(req.Context(), platform.CtxProjectID, int64(1)))
	api.Executions(rr, req)
	if rr.Code != 200 {
		t.Fatalf("exec %d %s", rr.Code, rr.Body.String())
	}

	// recipients via role
	mock.ExpectQuery("SELECT u.email FROM project_members").
		WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("pm@local"))
	to, err := w.recipients(context.Background(), 1, Action{ToRole: domain.RolePM})
	if err != nil || len(to) != 1 {
		t.Fatalf("%v %v", to, err)
	}
	mock.ExpectQuery("SELECT u.email FROM project_members").
		WillReturnRows(sqlmock.NewRows([]string{"email"}))
	if _, err := w.recipients(context.Background(), 1, Action{}); ClassifyError(err) != domain.ErrorPermanent {
		t.Fatalf("empty recipients %v", err)
	}

	// transient action → retry outbox, no execution row
	w.Mailer = &fakeMailer{err: errors.New("connection reset")}
	pl := domain.StatusChangedPayload{ProjectID: 1, ToColumn: "DONE", EventID: "e-retry"}
	raw, _ := json.Marshal(pl)
	retryEv := &domain.OutboxEvent{ID: 8, EventID: "e-retry", EventType: domain.EventIssueStatusChanged, Payload: raw, RetryCount: 0}
	mock.ExpectQuery("FROM triggers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at"}).
			AddRow(4, 1, "pm mail", domain.EventIssueStatusChanged, []byte(`{"to_column":"DONE"}`),
				[]byte(`[{"type":"SEND_EMAIL","to":"pm@local"}]`), true, now))
	mock.ExpectQuery("COUNT\\(\\*\\) FROM trigger_executions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE outbox_events SET retry_count").WillReturnResult(sqlmock.NewResult(0, 1))
	w.deliver(context.Background(), retryEv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	(&Worker{Cfg: &config.Config{SMTPHost: "mailpit", SMTPPort: 1025}, Log: slog.Default()}).Start(ctx)
}

func TestSMTPSendUnreachable(t *testing.T) {
	err := SMTPMailer{Host: "127.0.0.1", Port: 1, From: "gojira@local.dev"}.Send([]string{"pm@local"}, "s", "b")
	if err == nil {
		t.Fatal("expected dial error")
	}
	if ClassifyError(err) == "" {
		t.Fatal("must classify")
	}
	err = SMTPMailer{Host: "127.0.0.1", Port: 1, From: "a@b.c", TLS: true}.Send([]string{"pm@local"}, "s", "b")
	if err == nil {
		t.Fatal("tls dial should fail")
	}
}

func TestDeliverBranches(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	w := &Worker{DB: sqlx.NewDb(db, "postgres"), Mailer: &fakeMailer{}, Log: slog.Default()}
	now := time.Now()
	pl := domain.StatusChangedPayload{ProjectID: 1, ToColumn: "DONE"}
	raw, _ := json.Marshal(pl)

	// condition miss → treat as success
	mock.ExpectQuery("FROM triggers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at"}).
			AddRow(1, 1, "x", domain.EventIssueStatusChanged, []byte(`{"to_column":"TODO"}`), []byte(`[]`), true, now))
	mock.ExpectExec("UPDATE outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	w.deliver(context.Background(), &domain.OutboxEvent{ID: 1, EventID: "a", EventType: domain.EventIssueStatusChanged, Payload: raw})

	// invalid actions json → dead after no retry
	mock.ExpectQuery("FROM triggers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "event_type", "condition", "actions", "is_enabled", "created_at"}).
			AddRow(1, 1, "x", domain.EventIssueStatusChanged, []byte(`{}`), []byte(`{`), true, now))
	mock.ExpectExec("UPDATE outbox_events").WillReturnResult(sqlmock.NewResult(0, 1))
	w.deliver(context.Background(), &domain.OutboxEvent{ID: 2, EventID: "b", EventType: domain.EventIssueStatusChanged, Payload: raw, RetryCount: 3})

	// permanent action recorded as skipped
	mock.ExpectQuery("COUNT\\(\\*\\) FROM trigger_executions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO trigger_executions").WillReturnResult(sqlmock.NewResult(1, 1))
	ok := w.execOnce(context.Background(), &domain.OutboxEvent{EventID: "c"}, domain.Trigger{ID: 1}, Action{Type: "NOPE"}, pl)
	if !ok {
		t.Fatal("permanent failure should not retry")
	}

	mock.ExpectQuery("FROM triggers WHERE").WillReturnError(errors.New("db down"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), platform.CtxProjectID, int64(1)))
	(&API{DB: w.DB}).List(rr, req)
	if rr.Code != 500 {
		t.Fatalf("list err %d", rr.Code)
	}
}
