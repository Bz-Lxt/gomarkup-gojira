package gantt

import (
	"net/http"

	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/jmoiron/sqlx"
)

type Edge struct {
	FromID int64 `db:"from_id"`
	ToID   int64 `db:"to_id"`
}

type Service struct {
	DB *sqlx.DB
}

func (s *Service) View(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	var issues []domain.Issue
	err := s.DB.Select(&issues, `
		SELECT i.*, (p.key || '-' || i.seq_no::text) AS issue_key, p.key AS project_key
		FROM issues i JOIN projects p ON p.id=i.project_id
		WHERE i.project_id=$1
		ORDER BY i.start_date NULLS LAST, i.id`, pid)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	today := platform.StartOfDay(platform.Now())
	bars := make([]domain.GanttBar, 0, len(issues))
	for _, iss := range issues {
		overdue := false
		if iss.DueDate != nil && iss.DueDate.Before(today) && !domain.IsDoneStatus(iss.Status) {
			overdue = true
		}
		bars = append(bars, domain.GanttBar{
			IssueID: iss.ID, Key: iss.Key, Title: iss.Title, IssueType: iss.IssueType,
			Status: iss.Status, StartDate: iss.StartDate, DueDate: iss.DueDate,
			Overdue: overdue, AssigneeID: iss.AssigneeID,
		})
	}
	var deps []domain.Dependency
	_ = s.DB.Select(&deps, `
		SELECT d.*,
			(pp.key || '-' || pred.seq_no::text) AS pred_key,
			(sp.key || '-' || succ.seq_no::text) AS succ_key
		FROM issue_dependencies d
		JOIN issues pred ON pred.id=d.predecessor_id
		JOIN issues succ ON succ.id=d.successor_id
		JOIN projects pp ON pp.id=pred.project_id
		JOIN projects sp ON sp.id=succ.project_id
		WHERE pred.project_id=$1 OR succ.project_id=$1
		ORDER BY d.id`, pid)
	if deps == nil {
		deps = []domain.Dependency{}
	}
	platform.WriteData(w, http.StatusOK, domain.GanttView{
		Bars: bars, Dependencies: deps, Today: today.Format("2006-01-02"),
	})
}

// DetectCycle reports whether adding from→to would introduce a directed cycle.
// It walks the existing graph from `to`; if `from` is reachable, the new edge closes a loop.
func DetectCycle(existing []Edge, from, to int64) (bool, []int64) {
	if from == to {
		return true, []int64{from, to}
	}
	adj := map[int64][]int64{}
	for _, e := range existing {
		adj[e.FromID] = append(adj[e.FromID], e.ToID)
	}
	parent := map[int64]int64{}
	seen := map[int64]bool{to: true}
	queue := []int64{to}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range adj[u] {
			if v == from {
				parent[v] = u
				return true, reconstruct(parent, from, to)
			}
			if seen[v] {
				continue
			}
			seen[v] = true
			parent[v] = u
			queue = append(queue, v)
		}
	}
	return false, nil
}

func reverse(in []int64) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

func reconstruct(parent map[int64]int64, from, to int64) []int64 {
	var back []int64
	for x := from; ; {
		back = append(back, x)
		if x == to {
			break
		}
		p, ok := parent[x]
		if !ok {
			break
		}
		x = p
	}
	return append([]int64{from}, reverse(back)...)
}
