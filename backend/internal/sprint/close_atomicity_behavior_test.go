package sprint_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gojira/internal/domain"
	"gojira/internal/platform"
	"gojira/internal/sprint"
	"gojira/internal/stats"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

var closeDriverID atomic.Uint64

func TestCloseRollsBackIssueMoveWhenOutboxWriteFails(t *testing.T) {
	state := &closeState{issueInSprint: true, sprintStatus: domain.SprintActive}
	driverName := fmt.Sprintf("sprint-close-atomicity-%d", closeDriverID.Add(1))
	sql.Register(driverName, &closeDriver{state: state})
	rawDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	db := sqlx.NewDb(rawDB, driverName)
	t.Cleanup(func() { _ = db.Close() })

	closeService := &sprint.Service{DB: db}
	closeRouter := chi.NewRouter()
	closeRouter.Post("/sprints/{id}/close", closeService.Close)
	closeRequest := httptest.NewRequest(http.MethodPost, "/sprints/1/close", strings.NewReader(`{"to_backlog":true}`))
	closeRequest = closeRequest.WithContext(context.WithValue(closeRequest.Context(), platform.CtxProjectRole, domain.RolePM))
	closeResponse := httptest.NewRecorder()
	closeRouter.ServeHTTP(closeResponse, closeRequest)
	if closeResponse.Code != http.StatusInternalServerError {
		t.Fatalf("close status = %d, want %d; body=%s", closeResponse.Code, http.StatusInternalServerError, closeResponse.Body.String())
	}

	progressService := &stats.Service{DB: db}
	progressRouter := chi.NewRouter()
	progressRouter.Get("/sprints/{id}/progress", progressService.Progress)
	progressResponse := httptest.NewRecorder()
	progressRouter.ServeHTTP(progressResponse, httptest.NewRequest(http.MethodGet, "/sprints/1/progress", nil))
	if progressResponse.Code != http.StatusOK {
		t.Fatalf("progress status = %d, want %d; body=%s", progressResponse.Code, http.StatusOK, progressResponse.Body.String())
	}
	var body struct {
		Data struct {
			ScopePoints float64 `json:"scope_points"`
		} `json:"data"`
	}
	if err := json.Unmarshal(progressResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode progress response: %v", err)
	}
	if body.Data.ScopePoints != 5 {
		t.Fatalf("scope_points = %v after failed close, want 5", body.Data.ScopePoints)
	}
}

type closeState struct {
	mu              sync.Mutex
	issueInSprint   bool
	sprintStatus    string
}

type closeDriver struct {
	state *closeState
}

func (d *closeDriver) Open(string) (driver.Conn, error) {
	return &closeConn{state: d.state}, nil
}

type closeConn struct {
	state *closeState
	tx    *closeTx
}

func (c *closeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *closeConn) Close() error                        { return nil }

func (c *closeConn) Begin() (driver.Tx, error) {
	return c.begin()
}

func (c *closeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.begin()
}

func (c *closeConn) begin() (driver.Tx, error) {
	if c.tx != nil {
		return nil, errors.New("transaction already active")
	}
	c.state.mu.Lock()
	tx := &closeTx{
		conn:            c,
		issueInSprint:   c.state.issueInSprint,
		sprintStatus:    c.state.sprintStatus,
	}
	c.state.mu.Unlock()
	c.tx = tx
	return tx, nil
}

func (c *closeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	normalized := strings.ToUpper(query)
	switch {
	case strings.Contains(normalized, "UPDATE ISSUES SET SPRINT_ID"):
		inSprint := len(args) > 0 && args[0].Value != nil
		if c.tx != nil {
			c.tx.issueInSprint = inSprint
		} else {
			c.state.mu.Lock()
			c.state.issueInSprint = inSprint
			c.state.mu.Unlock()
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(normalized, "UPDATE SPRINTS SET STATUS='CLOSED'"):
		if c.tx != nil {
			c.tx.sprintStatus = domain.SprintClosed
		} else {
			c.state.mu.Lock()
			c.state.sprintStatus = domain.SprintClosed
			c.state.mu.Unlock()
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(normalized, "INSERT INTO OUTBOX_EVENTS"):
		return nil, errors.New("outbox unavailable")
	default:
		return nil, fmt.Errorf("unexpected exec: %s", strings.TrimSpace(query))
	}
}

func (c *closeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToUpper(query)
	switch {
	case strings.Contains(normalized, "WITH SP AS") && strings.Contains(normalized, "COMPLETED_POINTS"):
		return c.progressRows(), nil
	case strings.Contains(normalized, "SELECT * FROM SPRINTS WHERE ID"):
		return c.sprintRows(), nil
	default:
		return nil, fmt.Errorf("unexpected query: %s", strings.TrimSpace(query))
	}
}

func (c *closeConn) sprintRows() driver.Rows {
	c.state.mu.Lock()
	status := c.state.sprintStatus
	c.state.mu.Unlock()
	start := time.Date(2026, time.August, 17, 0, 0, 0, 0, platform.Location())
	return &closeRows{
		columns: []string{"id", "project_id", "name", "goal", "start_date", "end_date", "status", "committed_points", "created_at", "updated_at"},
		values: [][]driver.Value{{
			int64(1), int64(1), "Sprint 1", "", start, start.AddDate(0, 0, 13), status, int64(5), start, start,
		}},
	}
}

func (c *closeConn) progressRows() driver.Rows {
	c.state.mu.Lock()
	inSprint := c.state.issueInSprint
	c.state.mu.Unlock()
	scope := float64(0)
	count := int64(0)
	if inSprint {
		scope = 5
		count = 1
	}
	start := time.Date(2026, time.August, 17, 0, 0, 0, 0, platform.Location())
	return &closeRows{
		columns: []string{
			"committed_points", "scope_points", "completed_points", "issue_count",
			"completed_count", "start_date", "end_date",
		},
		values: [][]driver.Value{{
			float64(5), scope, float64(0), count, int64(0), start, start.AddDate(0, 0, 13),
		}},
	}
}

type closeTx struct {
	conn            *closeConn
	issueInSprint   bool
	sprintStatus    string
	done            bool
}

func (tx *closeTx) Commit() error {
	if tx.done {
		return errors.New("transaction already finished")
	}
	tx.conn.state.mu.Lock()
	tx.conn.state.issueInSprint = tx.issueInSprint
	tx.conn.state.sprintStatus = tx.sprintStatus
	tx.conn.state.mu.Unlock()
	tx.done = true
	tx.conn.tx = nil
	return nil
}

func (tx *closeTx) Rollback() error {
	if tx.done {
		return errors.New("transaction already finished")
	}
	tx.done = true
	tx.conn.tx = nil
	return nil
}

type closeRows struct {
	columns []string
	values  [][]driver.Value
	next    int
}

func (r *closeRows) Columns() []string { return r.columns }
func (r *closeRows) Close() error      { return nil }

func (r *closeRows) Next(dest []driver.Value) error {
	if r.next >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.next])
	r.next++
	return nil
}
