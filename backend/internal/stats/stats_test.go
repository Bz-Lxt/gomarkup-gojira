package stats

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func TestQueriesNonEmpty(t *testing.T) {
	qs := []string{BurndownSQL, VelocitySQL, ProgressSQL, BugStatsSQL, SnapshotSQL}
	for i, q := range qs {
		if strings.TrimSpace(q) == "" {
			t.Fatalf("query %d is empty", i)
		}
		if !strings.Contains(strings.ToLower(q), "select") && !strings.Contains(strings.ToLower(q), "insert") {
			t.Fatalf("query %d missing select/insert", i)
		}
	}
	if !strings.Contains(BurndownSQL, "generate_series") {
		t.Fatal("burndown must use generate_series")
	}
	if !strings.Contains(BurndownSQL, "issue_status_history") {
		t.Fatal("burndown must use issue_status_history")
	}
}

func TestRemainingOnTable(t *testing.T) {
	loc := platform.Location()
	d0 := time.Date(2026, 8, 17, 0, 0, 0, 0, loc)
	d1 := d0.Add(24 * time.Hour)
	d2 := d0.Add(48 * time.Hour)
	issues := []IssueSnapshot{
		{
			ID: 1, StoryPoints: 5, EstimateHours: 8, Status: "DONE", EnteredOn: d0,
			History: []HistoryEntry{
				{ChangedAt: d0.Add(10 * time.Hour), ToStatus: "IN_PROGRESS"},
				{ChangedAt: d1.Add(15 * time.Hour), ToStatus: "DONE"},
			},
		},
		{
			ID: 2, StoryPoints: 3, EstimateHours: 4, Status: "TODO", EnteredOn: d1,
		},
		{
			ID: 3, StoryPoints: 8, EstimateHours: 16, Status: "IN_PROGRESS", EnteredOn: d0,
			History: []HistoryEntry{{ChangedAt: d0.Add(2 * time.Hour), ToStatus: "IN_PROGRESS"}},
		},
	}
	tests := []struct {
		name       string
		day        time.Time
		metric     string
		wantRemain float64
		wantScope  float64
	}{
		{"day0 points: #1 in progress + #3, #2 not yet in scope", d0, "points", 13, 13},
		{"day1 points: #1 done, #2+#3 remaining", d1, "points", 11, 16},
		{"day2 same as day1", d2, "points", 11, 16},
		{"day0 count", d0, "count", 2, 2},
		{"day1 hours: 4+16 (issue1 done)", d1, "hours", 20, 28},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotR, gotS := RemainingOn(RemainingInput{Day: tt.day, Issues: issues, Metric: tt.metric, DoneSet: domain.DoneStatuses})
			if gotR != tt.wantRemain || gotS != tt.wantScope {
				t.Fatalf("remain/scope = %v/%v want %v/%v", gotR, gotS, tt.wantRemain, tt.wantScope)
			}
		})
	}
}

func TestIdealLine(t *testing.T) {
	if IdealLine(10, 0, 5) != 10 {
		t.Fatalf("start should be committed")
	}
	if IdealLine(10, 4, 5) != 0 {
		t.Fatalf("last day should be 0")
	}
	mid := IdealLine(10, 2, 5)
	if mid < 4.9 || mid > 5.1 {
		t.Fatalf("mid = %v", mid)
	}
}

func TestNormalizeMetric(t *testing.T) {
	if NormalizeMetric("HOURS") != "hours" {
		t.Fatal()
	}
	if NormalizeMetric("nope") != "points" {
		t.Fatal()
	}
}

func TestIdealLineEdges(t *testing.T) {
	if IdealLine(8, 0, 1) != 0 {
		t.Fatal("single day")
	}
	if IdealLine(8, -1, 5) != IdealLine(8, 0, 5) {
		t.Fatal("neg index clamps")
	}
	if IdealLine(8, 9, 5) != 0 {
		t.Fatal("past end")
	}
}

func TestComputeProgress(t *testing.T) {
	loc := platform.Location()
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, loc)
	end := time.Date(2026, 8, 30, 0, 0, 0, 0, loc)
	today := time.Date(2026, 8, 21, 0, 0, 0, 0, loc)
	p := ComputeProgress(ProgressInput{
		CommittedPoints: 34, ScopePoints: 40, CompletedPoints: 16,
		IssueCount: 12, CompletedCount: 4,
		StartDate: start, EndDate: end, Today: today,
	})
	if p.RemainingDays != 9 {
		t.Fatalf("remain days %d", p.RemainingDays)
	}
	if p.CompletionRate < 0.3 || p.CompletionRate > 0.5 {
		t.Fatalf("rate %v", p.CompletionRate)
	}
	if p.Velocity <= 0 || p.PredictedDoneOn == "" {
		t.Fatalf("velocity/pred %v %s", p.Velocity, p.PredictedDoneOn)
	}
	done := ComputeProgress(ProgressInput{
		ScopePoints: 10, CompletedPoints: 10, StartDate: start, EndDate: end, Today: today,
	})
	if done.PredictedDoneOn != today.Format("2006-01-02") {
		t.Fatalf("done pred %s", done.PredictedDoneOn)
	}
	past := ComputeProgress(ProgressInput{
		ScopePoints: 10, CompletedPoints: 2, StartDate: start, EndDate: start.AddDate(0, 0, 1), Today: today,
	})
	if past.RemainingDays != 0 {
		t.Fatalf("past sprint remain %d", past.RemainingDays)
	}
	sameDay := ComputeProgress(ProgressInput{
		ScopePoints: 8, CompletedPoints: 0, StartDate: today, EndDate: end, Today: today,
	})
	if sameDay.Velocity != 0 {
		t.Fatalf("no completed → velocity 0, got %v", sameDay.Velocity)
	}
}

func TestAttachIdeal(t *testing.T) {
	rows := AttachIdeal([]DayPoint{
		{Day: "2026-08-17", Committed: 10, Remaining: 10},
		{Day: "2026-08-18", Committed: 10, Remaining: 7},
		{Day: "2026-08-19", Committed: 10, Remaining: 0},
	})
	if rows[0].Ideal != 10 || rows[2].Ideal != 0 {
		t.Fatalf("%+v", rows)
	}
	empty := AttachIdeal(nil)
	if len(empty) != 0 {
		t.Fatal("empty")
	}
}

func TestScopeChangeIncreasesRemaining(t *testing.T) {
	loc := platform.Location()
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, loc)
	added := start.Add(72 * time.Hour)
	issues := []IssueSnapshot{
		{ID: 1, StoryPoints: 5, Status: "TODO", EnteredOn: start},
		{ID: 2, StoryPoints: 8, Status: "TODO", EnteredOn: added},
	}
	r0, s0 := RemainingOn(RemainingInput{Day: start, Issues: issues, Metric: "points"})
	r3, s3 := RemainingOn(RemainingInput{Day: added, Issues: issues, Metric: "points"})
	if r0 != 5 || s0 != 5 {
		t.Fatalf("before scope change %v/%v", r0, s0)
	}
	if r3 != 13 || s3 != 13 {
		t.Fatalf("after scope change %v/%v", r3, s3)
	}
}

func TestHandlersWithSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := &Service{DB: sqlx.NewDb(db, "postgres"), Log: slog.Default()}

	day := time.Date(2026, 8, 17, 0, 0, 0, 0, platform.Location())
	mock.ExpectQuery("generate_series").
		WithArgs(int64(1), "points").
		WillReturnRows(sqlmock.NewRows([]string{"day", "remaining", "scope", "committed"}).
			AddRow(day, 10.0, 10.0, 10.0).
			AddRow(day.Add(24*time.Hour), 5.0, 10.0, 10.0))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sprints/1/burndown?metric=points", nil)
	r := chi.NewRouter()
	r.Get("/sprints/{id}/burndown", s.Burndown)
	r.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("burndown %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sprints/1/burndown?metric=nope", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("bad metric %d", rr.Code)
	}

	mock.ExpectQuery("date_trunc").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"iso_week", "week_start", "assignee_id", "assignee_name", "points", "issue_count"}).
			AddRow("2026-34", day, 2, "dev", 8.0, 2))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/projects/1/velocity", nil)
	req = req.WithContext(context.WithValue(req.Context(), platform.CtxProjectID, int64(1)))
	s.Velocity(rr, req)
	if rr.Code != 200 {
		t.Fatalf("velocity %d %s", rr.Code, rr.Body.String())
	}

	mock.ExpectQuery("committed_points").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"committed_points", "scope_points", "completed_points", "issue_count", "completed_count", "start_date", "end_date"}).
			AddRow(34.0, 34.0, 10.0, 12, 3, day, day.AddDate(0, 0, 13)))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sprints/1/progress", nil)
	r2 := chi.NewRouter()
	r2.Get("/sprints/{id}/progress", s.Progress)
	r2.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("progress %d %s", rr.Code, rr.Body.String())
	}

	mock.ExpectQuery("severity_dist").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"severity_dist", "status_dist", "mttr_hours", "total"}).
			AddRow([]byte(`{"MAJOR":1}`), []byte(`{"NEW":1}`), 12.5, 1))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/projects/1/bug-stats", nil)
	req = req.WithContext(context.WithValue(req.Context(), platform.CtxProjectID, int64(1)))
	s.BugStats(rr, req)
	if rr.Code != 200 {
		t.Fatalf("bugs %d %s", rr.Code, rr.Body.String())
	}

	mock.ExpectExec("sprint_daily_snapshots").
		WithArgs(int64(3), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := s.WriteSnapshot(context.Background(), 3, platform.Now()); err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id FROM sprints").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectExec("sprint_daily_snapshots").
		WithArgs(int64(3), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.snapshotAll(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.SnapshotLoop(ctx)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sprints/x/burndown", nil)
	r.ServeHTTP(rr, req)
	if rr.Code == 200 {
		t.Fatal("invalid id")
	}

	mock.ExpectQuery("committed_points").
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sprints/99/progress", nil)
	r2.ServeHTTP(rr, req)
	if rr.Code != 404 {
		t.Fatalf("missing sprint %d", rr.Code)
	}
}
