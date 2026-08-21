package sprint

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gojira/internal/domain"
	"gojira/internal/issue"
	"gojira/internal/platform"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	DB *sqlx.DB
}

type createBody struct {
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type assignBody struct {
	IssueIDs []int64 `json:"issue_ids"`
	IssueID  int64   `json:"issue_id"`
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	var rows []domain.Sprint
	if err := s.DB.Select(&rows, `SELECT * FROM sprints WHERE project_id=$1 ORDER BY start_date DESC`, pid); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	role := platform.ProjectRoleFrom(r)
	if role != domain.RoleAdmin && role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("仅 PM/ADMIN 可创建迭代", nil))
		return
	}
	var body createBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "迭代名称必填", nil))
		return
	}
	start, err := platform.ParseDate(body.StartDate)
	if err != nil {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "开始日期非法", body.StartDate))
		return
	}
	end, err := platform.ParseDate(body.EndDate)
	if err != nil {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "结束日期非法", body.EndDate))
		return
	}
	if !end.After(start) {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "结束日期必须晚于开始日期", nil))
		return
	}
	now := platform.Now()
	var sp domain.Sprint
	err = s.DB.QueryRowx(`
		INSERT INTO sprints (project_id, name, goal, start_date, end_date, status, committed_points, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'PLANNED',0,$6,$6)
		RETURNING id, project_id, name, goal, start_date, end_date, status, committed_points, created_at, updated_at`,
		platform.ProjectIDFrom(r), body.Name, body.Goal, start, end, now,
	).StructScan(&sp)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusCreated, sp)
}

func (s *Service) Start(w http.ResponseWriter, r *http.Request) {
	if role := platform.ProjectRoleFrom(r); role != domain.RoleAdmin && role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("仅 PM/ADMIN 可启动迭代", nil))
		return
	}
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var sp domain.Sprint
	if err := s.DB.Get(&sp, `SELECT * FROM sprints WHERE id=$1`, id); err != nil {
		platform.WriteError(w, r, domain.NotFound("迭代不存在"))
		return
	}
	if sp.Status == domain.SprintActive {
		platform.WriteData(w, http.StatusOK, sp)
		return
	}
	if sp.Status != domain.SprintPlanned {
		platform.WriteError(w, r, domain.Conflict(domain.CodeConflict, "仅计划中的迭代可启动", sp.Status))
		return
	}
	var active int
	_ = s.DB.Get(&active, `SELECT COUNT(*) FROM sprints WHERE project_id=$1 AND status='ACTIVE'`, sp.ProjectID)
	if active > 0 {
		platform.WriteError(w, r, domain.Conflict(domain.CodeConflict, "同项目同时只能有一个进行中的迭代", nil))
		return
	}
	var points int
	_ = s.DB.Get(&points, `SELECT COALESCE(SUM(story_points),0) FROM issues WHERE sprint_id=$1`, id)
	now := platform.Now()
	tx, err := s.DB.Beginx()
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE sprints SET status='ACTIVE', committed_points=$1, updated_at=$2 WHERE id=$3`, points, now, id); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	eventID := issue.NewEventID("sprint.started")
	raw, _ := json.Marshal(map[string]any{"sprint_id": id, "project_id": sp.ProjectID, "committed_points": points})
	if _, err := tx.Exec(`INSERT INTO outbox_events (event_id, event_type, payload, status, created_at, updated_at)
		VALUES ($1,$2,$3,'pending',$4,$4)`, eventID, domain.EventSprintStarted, raw, now); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	if err := tx.Commit(); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	_ = s.DB.Get(&sp, `SELECT * FROM sprints WHERE id=$1`, id)
	platform.WriteData(w, http.StatusOK, sp)
}

func (s *Service) Close(w http.ResponseWriter, r *http.Request) {
	if role := platform.ProjectRoleFrom(r); role != domain.RoleAdmin && role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("仅 PM/ADMIN 可关闭迭代", nil))
		return
	}
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var dest struct {
		MoveToSprint *int64 `json:"move_to_sprint_id"`
		ToBacklog    bool   `json:"to_backlog"`
	}
	_ = platform.DecodeJSON(r, &dest)
	var sp domain.Sprint
	if err := s.DB.Get(&sp, `SELECT * FROM sprints WHERE id=$1`, id); err != nil {
		platform.WriteError(w, r, domain.NotFound("迭代不存在"))
		return
	}
	now := platform.Now()
	tx, err := s.DB.Beginx()
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	defer func() { _ = tx.Rollback() }()
	if dest.ToBacklog || dest.MoveToSprint != nil {
		q := `UPDATE issues SET sprint_id=$1, updated_at=$2 WHERE sprint_id=$3 AND status NOT IN ('DONE','RESOLVED','CLOSED','REJECTED')`
		var target any
		if dest.ToBacklog {
			target = nil
		} else {
			target = *dest.MoveToSprint
		}
		if _, err := tx.Exec(q, target, now, id); err != nil {
			platform.WriteError(w, r, domain.Internal(err))
			return
		}
	}
	if _, err := tx.Exec(`UPDATE sprints SET status='CLOSED', updated_at=$1 WHERE id=$2`, now, id); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	eventID := issue.NewEventID("sprint.closed")
	raw, _ := json.Marshal(map[string]any{"sprint_id": id, "project_id": sp.ProjectID})
	if _, err := tx.Exec(`INSERT INTO outbox_events (event_id, event_type, payload, status, created_at, updated_at)
		VALUES ($1,$2,$3,'pending',$4,$4)`, eventID, domain.EventSprintClosed, raw, now); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	if err := tx.Commit(); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	_ = s.DB.Get(&sp, `SELECT * FROM sprints WHERE id=$1`, id)
	platform.WriteData(w, http.StatusOK, sp)
}

func (s *Service) AssignIssues(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var body assignBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var sp domain.Sprint
	if err := s.DB.Get(&sp, `SELECT * FROM sprints WHERE id=$1`, id); err != nil {
		platform.WriteError(w, r, domain.NotFound("迭代不存在"))
		return
	}
	ids := append([]int64{}, body.IssueIDs...)
	if body.IssueID > 0 {
		ids = append(ids, body.IssueID)
	}
	now := platform.Now()
	for _, iid := range ids {
		if _, err := s.DB.Exec(`UPDATE issues SET sprint_id=$1, updated_at=$2 WHERE id=$3 AND project_id=$4`,
			id, now, iid, sp.ProjectID); err != nil {
			platform.WriteError(w, r, domain.Internal(err))
			return
		}
	}
	platform.WriteData(w, http.StatusOK, map[string]any{"sprint_id": id, "assigned": len(ids)})
}

func (s *Service) UnassignIssue(w http.ResponseWriter, r *http.Request) {
	sid, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	iid, err := platform.ParseID(chi.URLParam(r, "issueID"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if _, err := s.DB.Exec(`UPDATE issues SET sprint_id=NULL, updated_at=$1 WHERE id=$2 AND sprint_id=$3`,
		platform.Now(), iid, sid); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, map[string]any{"sprint_id": sid, "issue_id": iid})
}

func NextSnapshotAt(now time.Time) time.Time {
	now = now.In(platform.Location())
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, platform.Location())
	if !now.Before(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
