package stats

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"gojira/internal/domain"
	"gojira/internal/platform"
	"gojira/internal/sprint"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	DB  *sqlx.DB
	Log *slog.Logger
}

type DayPoint struct {
	Day       string  `db:"day" json:"day"`
	Remaining float64 `db:"remaining" json:"remaining"`
	Scope     float64 `db:"scope" json:"scope"`
	Committed float64 `db:"committed" json:"committed"`
	Ideal     float64 `json:"ideal"`
}

type HistoryEntry struct {
	ChangedAt time.Time
	ToStatus  string
}

type IssueSnapshot struct {
	ID            int64
	StoryPoints   int
	EstimateHours float64
	Status        string
	EnteredOn     time.Time
	History       []HistoryEntry
}

type RemainingInput struct {
	Day     time.Time
	Issues  []IssueSnapshot
	Metric  string
	DoneSet map[string]struct{}
}

// RemainingOn computes remaining work as of the end of day (pure; used by tests).
func RemainingOn(in RemainingInput) (remaining, scope float64) {
	done := in.DoneSet
	if done == nil {
		done = domain.DoneStatuses
	}
	for _, iss := range in.Issues {
		if iss.EnteredOn.After(in.Day) {
			continue
		}
		status := iss.Status
		for _, h := range iss.History {
			if !h.ChangedAt.After(in.Day.Add(24*time.Hour - time.Nanosecond)) {
				status = h.ToStatus
			}
		}
		var w float64
		switch in.Metric {
		case "hours":
			w = iss.EstimateHours
		case "count":
			w = 1
		default:
			w = float64(iss.StoryPoints)
		}
		scope += w
		if _, ok := done[status]; !ok {
			remaining += w
		}
	}
	return remaining, scope
}

func IdealLine(committed float64, dayIndex, totalDays int) float64 {
	if totalDays <= 1 {
		return 0
	}
	if dayIndex < 0 {
		dayIndex = 0
	}
	if dayIndex >= totalDays {
		return 0
	}
	return committed * (1 - float64(dayIndex)/float64(totalDays-1))
}

type VelocityRow struct {
	ISOWeek      string    `db:"iso_week" json:"week"`
	WeekStart    time.Time `db:"week_start" json:"week_start"`
	AssigneeID   *int64    `db:"assignee_id" json:"user_id"`
	AssigneeName string    `db:"assignee_name" json:"display_name"`
	Points       float64   `db:"points" json:"points"`
	IssueCount   int64     `db:"issue_count" json:"count"`
}

type ProgressView struct {
	CommittedPoints float64 `json:"committed_points"`
	ScopePoints     float64 `json:"scope_points"`
	CompletedPoints float64 `json:"completed_points"`
	CompletionRate  float64 `json:"completion_rate"`
	RemainingDays   int     `json:"remaining_days"`
	Velocity        float64 `json:"velocity"`
	PredictedDoneOn string  `json:"predicted_end"`
	IssueCount      int64   `json:"issue_count"`
	CompletedCount  int64   `json:"completed_count"`
}

type ProgressInput struct {
	CommittedPoints float64
	ScopePoints     float64
	CompletedPoints float64
	IssueCount      int64
	CompletedCount  int64
	StartDate       time.Time
	EndDate         time.Time
	Today           time.Time
}

func ComputeProgress(in ProgressInput) ProgressView {
	today := platform.StartOfDay(in.Today)
	remainDays := int(in.EndDate.Sub(today).Hours() / 24)
	if remainDays < 0 {
		remainDays = 0
	}
	elapsed := today.Sub(platform.StartOfDay(in.StartDate)).Hours() / 24
	if elapsed < 1 {
		elapsed = 1
	}
	velocity := in.CompletedPoints / elapsed * 7
	rate := 0.0
	if in.ScopePoints > 0 {
		rate = in.CompletedPoints / in.ScopePoints
	}
	left := in.ScopePoints - in.CompletedPoints
	pred := ""
	if velocity > 0 && left > 0 {
		days := left / (velocity / 7)
		pred = today.Add(time.Duration(days*24) * time.Hour).Format("2006-01-02")
	} else if left <= 0 {
		pred = today.Format("2006-01-02")
	}
	return ProgressView{
		CommittedPoints: in.CommittedPoints, ScopePoints: in.ScopePoints,
		CompletedPoints: in.CompletedPoints, CompletionRate: math.Round(rate*1000) / 1000,
		RemainingDays: remainDays, Velocity: math.Round(velocity*100) / 100,
		PredictedDoneOn: pred, IssueCount: in.IssueCount, CompletedCount: in.CompletedCount,
	}
}

func AttachIdeal(rows []DayPoint) []DayPoint {
	n := len(rows)
	var committed float64
	if n > 0 {
		committed = rows[0].Committed
	}
	for i := range rows {
		rows[i].Ideal = math.Round(IdealLine(committed, i, n)*100) / 100
	}
	return rows
}

type BugStatsView struct {
	SeverityDist map[string]int `json:"by_severity"`
	StatusDist   map[string]int `json:"by_status"`
	MTTRHours    float64        `json:"mttr_hours"`
	Total        int64          `json:"total"`
}

func (s *Service) Burndown(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "points"
	}
	if metric != "points" && metric != "hours" && metric != "count" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "metric 须为 points|hours|count", metric))
		return
	}
	var scanned []struct {
		Day       time.Time `db:"day"`
		Remaining float64   `db:"remaining"`
		Scope     float64   `db:"scope"`
		Committed float64   `db:"committed"`
	}
	if err := s.DB.Select(&scanned, BurndownSQL, id, metric); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	n := len(scanned)
	rows := make([]DayPoint, n)
	for i, row := range scanned {
		rows[i] = DayPoint{
			Day:       row.Day.In(platform.Location()).Format("2006-01-02"),
			Remaining: row.Remaining,
			Scope:     row.Scope,
			Committed: row.Committed,
		}
	}
	rows = AttachIdeal(rows)
	ideal := make([]map[string]any, 0, n)
	actual := make([]map[string]any, 0, n)
	scopes := make([]map[string]any, 0)
	var prevScope float64
	for i, row := range rows {
		ideal = append(ideal, map[string]any{"date": row.Day, "value": row.Ideal})
		actual = append(actual, map[string]any{"date": row.Day, "value": row.Remaining})
		if i > 0 && row.Scope > prevScope+0.001 {
			scopes = append(scopes, map[string]any{"date": row.Day, "delta": row.Scope - prevScope})
		}
		prevScope = row.Scope
	}
	if scopes == nil {
		scopes = []map[string]any{}
	}
	platform.WriteData(w, http.StatusOK, map[string]any{
		"metric":        metric,
		"ideal":         ideal,
		"actual":        actual,
		"scope_changes": scopes,
	})
}

func (s *Service) Velocity(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	var rows []VelocityRow
	if err := s.DB.Select(&rows, VelocitySQL, pid); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (s *Service) Progress(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var raw struct {
		CommittedPoints float64   `db:"committed_points"`
		ScopePoints     float64   `db:"scope_points"`
		CompletedPoints float64   `db:"completed_points"`
		IssueCount      int64     `db:"issue_count"`
		CompletedCount  int64     `db:"completed_count"`
		StartDate       time.Time `db:"start_date"`
		EndDate         time.Time `db:"end_date"`
	}
	if err := s.DB.Get(&raw, ProgressSQL, id); err != nil {
		platform.WriteError(w, r, domain.NotFound("迭代不存在"))
		return
	}
	platform.WriteData(w, http.StatusOK, ComputeProgress(ProgressInput{
		CommittedPoints: raw.CommittedPoints, ScopePoints: raw.ScopePoints,
		CompletedPoints: raw.CompletedPoints, IssueCount: raw.IssueCount,
		CompletedCount: raw.CompletedCount, StartDate: raw.StartDate,
		EndDate: raw.EndDate, Today: platform.Now(),
	}))
}

func (s *Service) BugStats(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	var raw struct {
		Severity []byte  `db:"severity_dist"`
		Status   []byte  `db:"status_dist"`
		MTTR     float64 `db:"mttr_hours"`
		Total    int64   `db:"total"`
	}
	if err := s.DB.Get(&raw, BugStatsSQL, pid); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	view := BugStatsView{SeverityDist: map[string]int{}, StatusDist: map[string]int{}, MTTRHours: raw.MTTR, Total: raw.Total}
	_ = json.Unmarshal(raw.Severity, &view.SeverityDist)
	_ = json.Unmarshal(raw.Status, &view.StatusDist)
	platform.WriteData(w, http.StatusOK, view)
}

func (s *Service) WriteSnapshot(ctx context.Context, sprintID int64, day time.Time) error {
	_, err := s.DB.ExecContext(ctx, SnapshotSQL, sprintID, day.In(platform.Location()).Format("2006-01-02"))
	return err
}

func (s *Service) SnapshotLoop(ctx context.Context) {
	for {
		now := platform.Now()
		next := sprint.NextSnapshotAt(now)
		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.snapshotAll(ctx)
		}
	}
}

func (s *Service) snapshotAll(ctx context.Context) {
	var ids []int64
	if err := s.DB.SelectContext(ctx, &ids, `SELECT id FROM sprints WHERE status='ACTIVE'`); err != nil {
		s.Log.Error("snapshot list sprints", "err", err)
		return
	}
	day := platform.StartOfDay(platform.Now())
	for _, id := range ids {
		if err := s.WriteSnapshot(ctx, id, day); err != nil {
			s.Log.Error("snapshot write", "sprint", id, "err", err)
		}
	}
}

func NormalizeMetric(m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	switch m {
	case "hours", "count", "points":
		return m
	default:
		return "points"
	}
}
