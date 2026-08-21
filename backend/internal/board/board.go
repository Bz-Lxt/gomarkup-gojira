package board

import (
	"net/http"
	"strings"

	"gojira/internal/domain"
	"gojira/internal/platform"
	"gojira/internal/workflow"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	DB     *sqlx.DB
	Engine *workflow.Engine
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	q := r.URL.Query()
	args := []any{pid}
	where := []string{"i.project_id=$1"}
	n := 2
	if v := q.Get("assignee_id"); v != "" {
		where = append(where, "i.assignee_id=$"+itoa(n))
		args = append(args, v)
		n++
	}
	if v := q.Get("issue_type"); v != "" {
		where = append(where, "i.issue_type=$"+itoa(n))
		args = append(args, v)
		n++
	}
	if v := q.Get("label"); v != "" {
		where = append(where, `EXISTS (SELECT 1 FROM issue_labels l WHERE l.issue_id=i.id AND l.label=$`+itoa(n)+`)`)
		args = append(args, v)
		n++
	}
	if q.Get("mine") == "1" {
		u := platform.UserFrom(r)
		where = append(where, "i.assignee_id=$"+itoa(n))
		args = append(args, u.ID)
		n++
	}
	var issues []domain.Issue
	err := s.DB.Select(&issues, `
		SELECT i.*, (p.key || '-' || i.seq_no::text) AS issue_key, p.key AS project_key
		FROM issues i JOIN projects p ON p.id=i.project_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY i.board_rank, i.id`, args...)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	cols := s.Engine.Columns()
	view := domain.BoardView{Columns: make([]domain.BoardColumn, 0, len(cols))}
	index := map[string]int{}
	for i, c := range cols {
		view.Columns = append(view.Columns, domain.BoardColumn{
			ID: c.ID, Label: c.Label, Hint: c.Hint, Cards: []domain.Issue{},
		})
		index[c.ID] = i
	}
	for _, iss := range issues {
		col, ok := s.Engine.ColumnOf(iss.IssueType, iss.Status)
		if !ok {
			continue
		}
		idx, ok := index[col]
		if !ok {
			continue
		}
		view.Columns[idx].Cards = append(view.Columns[idx].Cards, iss)
		view.Columns[idx].Count++
		if iss.StoryPoints != nil {
			view.Columns[idx].Points += *iss.StoryPoints
		}
	}
	platform.WriteData(w, http.StatusOK, view)
}

func itoa(n int) string { return domain.FormatKey("", n)[1:] }
