package issue

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gojira/internal/domain"
	"gojira/internal/gantt"
	"gojira/internal/platform"
	"gojira/internal/workflow"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

const issueSelect = `
	SELECT i.*, (p.key || '-' || i.seq_no::text) AS issue_key, p.key AS project_key
	FROM issues i JOIN projects p ON p.id=i.project_id`

type Service struct {
	DB     *sqlx.DB
	Engine *workflow.Engine
}

type createBody struct {
	IssueType      string   `json:"issue_type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Priority       string   `json:"priority"`
	Severity       *string  `json:"severity"`
	ReproduceSteps *string  `json:"reproduce_steps"`
	AffectVersion  *string  `json:"affect_version"`
	FixVersion     *string  `json:"fix_version"`
	AssigneeID     *int64   `json:"assignee_id"`
	SprintID       *int64   `json:"sprint_id"`
	StoryPoints    *int     `json:"story_points"`
	EstimateHours  *float64 `json:"estimate_hours"`
	StartDate      *string  `json:"start_date"`
	DueDate        *string  `json:"due_date"`
	Labels         []string `json:"labels"`
}

type patchBody struct {
	Title          *string  `json:"title"`
	Description    *string  `json:"description"`
	Priority       *string  `json:"priority"`
	Severity       *string  `json:"severity"`
	ReproduceSteps *string  `json:"reproduce_steps"`
	AffectVersion  *string  `json:"affect_version"`
	FixVersion     *string  `json:"fix_version"`
	AssigneeID     *int64   `json:"assignee_id"`
	ClearAssignee  bool     `json:"clear_assignee"`
	SprintID       *int64   `json:"sprint_id"`
	ClearSprint    bool     `json:"clear_sprint"`
	StoryPoints    *int     `json:"story_points"`
	EstimateHours  *float64 `json:"estimate_hours"`
	StartDate      *string  `json:"start_date"`
	DueDate        *string  `json:"due_date"`
	Labels         []string `json:"labels"`
	Version        *int     `json:"version"`
}

type transitionBody struct {
	To      string `json:"to"`
	Version int    `json:"version"`
}

type rankBody struct {
	PrevRank  *float64 `json:"prev_rank"`
	NextRank  *float64 `json:"next_rank"`
	BoardRank *float64 `json:"board_rank"`
	Version   int      `json:"version"`
}

type depBody struct {
	PredecessorID int64  `json:"predecessor_id"`
	SuccessorID   int64  `json:"successor_id"`
	DepType       string `json:"dep_type"`
}

func ValidStoryPointsPtr(n *int) bool {
	if n == nil {
		return true
	}
	return domain.ValidStoryPoints(*n)
}

func MidRank(prev, next *float64) float64 {
	switch {
	case prev != nil && next != nil:
		return (*prev + *next) / 2
	case prev != nil:
		return *prev + 1000
	case next != nil:
		return *next / 2
	default:
		return 1000
	}
}

func parseDatePtr(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	t, err := platform.ParseDate(strings.TrimSpace(*s))
	if err != nil {
		return nil, domain.BadRequest(domain.CodeInvalidInput, "日期须为 YYYY-MM-DD", *s)
	}
	return &t, nil
}

func (s *Service) get(ctx context.Context, id int64) (*domain.Issue, error) {
	var iss domain.Issue
	if err := s.DB.GetContext(ctx, &iss, issueSelect+` WHERE i.id=$1`, id); err != nil {
		return nil, domain.NotFound("事项不存在")
	}
	iss.Labels = s.labels(ctx, id)
	return &iss, nil
}

func (s *Service) labels(ctx context.Context, id int64) []string {
	var ls []string
	_ = s.DB.SelectContext(ctx, &ls, `SELECT label FROM issue_labels WHERE issue_id=$1 ORDER BY label`, id)
	return ls
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	pid := platform.ProjectIDFrom(r)
	page, per, offset := platform.ParsePage(r)
	q := r.URL.Query()
	args := []any{pid}
	where := []string{"i.project_id=$1"}
	n := 2
	issueType := q.Get("issue_type")
	if issueType == "" {
		issueType = q.Get("type")
	}
	if issueType != "" {
		where = append(where, "i.issue_type=$"+itoa(n))
		args = append(args, issueType)
		n++
	}
	if v := q.Get("status"); v != "" {
		where = append(where, "i.status=$"+itoa(n))
		args = append(args, v)
		n++
	}
	if v := q.Get("assignee_id"); v != "" {
		where = append(where, "i.assignee_id=$"+itoa(n))
		args = append(args, v)
		n++
	}
	if v := q.Get("sprint_id"); v != "" {
		if v == "none" {
			where = append(where, "i.sprint_id IS NULL")
		} else {
			where = append(where, "i.sprint_id=$"+itoa(n))
			args = append(args, v)
			n++
		}
	}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		where = append(where, "(i.title ILIKE $"+itoa(n)+" OR i.description ILIKE $"+itoa(n)+")")
		args = append(args, "%"+v+"%")
		n++
	}
	if v := q.Get("label"); v != "" {
		where = append(where, `EXISTS (SELECT 1 FROM issue_labels l WHERE l.issue_id=i.id AND l.label=$`+itoa(n)+`)`)
		args = append(args, v)
		n++
	}
	clause := strings.Join(where, " AND ")
	var total int64
	_ = s.DB.Get(&total, `SELECT COUNT(*) FROM issues i WHERE `+clause, args...)
	args = append(args, per, offset)
	var rows []domain.Issue
	err := s.DB.Select(&rows, issueSelect+` WHERE `+clause+` ORDER BY i.board_rank, i.id LIMIT $`+itoa(n)+` OFFSET $`+itoa(n+1), args...)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	s.attachLabels(r.Context(), rows)
	platform.WriteDataMeta(w, http.StatusOK, rows, platform.NewMeta(page, per, total))
}

func (s *Service) attachLabels(ctx context.Context, rows []domain.Issue) {
	if len(rows) == 0 {
		return
	}
	ids := make([]int64, len(rows))
	idx := map[int64]int{}
	for i := range rows {
		ids[i] = rows[i].ID
		idx[rows[i].ID] = i
		rows[i].Labels = []string{}
	}
	type pair struct {
		IssueID int64  `db:"issue_id"`
		Label   string `db:"label"`
	}
	var pairs []pair
	q, args, err := sqlx.In(`SELECT issue_id, label FROM issue_labels WHERE issue_id IN (?) ORDER BY label`, ids)
	if err != nil {
		return
	}
	if err := s.DB.SelectContext(ctx, &pairs, s.DB.Rebind(q), args...); err != nil {
		return
	}
	for _, p := range pairs {
		if i, ok := idx[p.IssueID]; ok {
			rows[i].Labels = append(rows[i].Labels, p.Label)
		}
	}
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	u := platform.UserFrom(r)
	pid := platform.ProjectIDFrom(r)
	var body createBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	body.IssueType = strings.ToUpper(strings.TrimSpace(body.IssueType))
	body.Title = strings.TrimSpace(body.Title)
	if !domain.ValidIssueType(body.IssueType) {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "issue_type 须为 STORY/TASK/BUG", body.IssueType))
		return
	}
	if body.Title == "" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "标题必填", nil))
		return
	}
	if body.Priority == "" {
		body.Priority = "MEDIUM"
	}
	body.Priority = strings.ToUpper(body.Priority)
	if _, ok := domain.Priorities[body.Priority]; !ok {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法优先级", body.Priority))
		return
	}
	if !ValidStoryPointsPtr(body.StoryPoints) {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "故事点须为斐波那契 0/1/2/3/5/8/13/21", body.StoryPoints))
		return
	}
	if body.IssueType == domain.TypeBug && body.Severity != nil {
		sv := strings.ToUpper(*body.Severity)
		if _, ok := domain.Severities[sv]; !ok {
			platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法严重级别", sv))
			return
		}
		body.Severity = &sv
	}
	start, err := parseDatePtr(body.StartDate)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	due, err := parseDatePtr(body.DueDate)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	status, err := s.Engine.InitialStatus(body.IssueType)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	now := platform.Now()
	tx, err := s.DB.Beginx()
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(r.Context(), `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, pid); err != nil {
		platform.WriteError(w, r, domain.NotFound("项目不存在"))
		return
	}
	var seq int
	if err := tx.GetContext(r.Context(), &seq, `SELECT COALESCE(MAX(seq_no),0)+1 FROM issues WHERE project_id=$1`, pid); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	var maxRank float64
	_ = tx.GetContext(r.Context(), &maxRank, `SELECT COALESCE(MAX(board_rank),0) FROM issues WHERE project_id=$1`, pid)
	rank := maxRank + 1000
	var id int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO issues (
			project_id, seq_no, issue_type, title, description, status, priority, severity,
			reproduce_steps, affect_version, fix_version, assignee_id, reporter_id, sprint_id,
			story_points, estimate_hours, start_date, due_date, board_rank, version,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,1,$20,$20
		) RETURNING id`,
		pid, seq, body.IssueType, body.Title, body.Description, status, body.Priority, body.Severity,
		body.ReproduceSteps, body.AffectVersion, body.FixVersion, body.AssigneeID, u.ID, body.SprintID,
		body.StoryPoints, body.EstimateHours, start, due, rank, now,
	).Scan(&id)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	for _, lb := range body.Labels {
		lb = strings.TrimSpace(lb)
		if lb == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO issue_labels (issue_id, label) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, lb); err != nil {
			platform.WriteError(w, r, domain.Internal(err))
			return
		}
	}
	eventID := NewEventID("issue.created")
	payload, _ := json.Marshal(map[string]any{"issue_id": id, "project_id": pid, "event_id": eventID})
	if _, err := tx.Exec(`INSERT INTO outbox_events (event_id, event_type, payload, status, created_at, updated_at)
		VALUES ($1,$2,$3,'pending',$4,$4)`, eventID, domain.EventIssueCreated, payload, now); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	if err := tx.Commit(); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	iss, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/issues/"+itoa(int(id)))
	platform.WriteData(w, http.StatusCreated, iss)
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	iss, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteData(w, http.StatusOK, iss)
}

func (s *Service) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	iss, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var body patchBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if body.Version != nil && *body.Version != iss.Version {
		platform.WriteError(w, r, domain.OptimisticLock())
		return
	}
	if body.Title != nil {
		iss.Title = strings.TrimSpace(*body.Title)
		if iss.Title == "" {
			platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "标题不能为空", nil))
			return
		}
	}
	if body.Description != nil {
		iss.Description = *body.Description
	}
	if body.Priority != nil {
		p := strings.ToUpper(*body.Priority)
		if _, ok := domain.Priorities[p]; !ok {
			platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法优先级", p))
			return
		}
		iss.Priority = p
	}
	if body.Severity != nil {
		sv := strings.ToUpper(*body.Severity)
		if _, ok := domain.Severities[sv]; !ok {
			platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法严重级别", sv))
			return
		}
		iss.Severity = &sv
	}
	if body.ReproduceSteps != nil {
		iss.ReproduceSteps = body.ReproduceSteps
	}
	if body.AffectVersion != nil {
		iss.AffectVersion = body.AffectVersion
	}
	if body.FixVersion != nil {
		iss.FixVersion = body.FixVersion
	}
	if body.ClearAssignee {
		iss.AssigneeID = nil
	} else if body.AssigneeID != nil {
		iss.AssigneeID = body.AssigneeID
	}
	if body.ClearSprint {
		iss.SprintID = nil
	} else if body.SprintID != nil {
		iss.SprintID = body.SprintID
	}
	if body.StoryPoints != nil {
		if !domain.ValidStoryPoints(*body.StoryPoints) {
			platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法故事点", *body.StoryPoints))
			return
		}
		iss.StoryPoints = body.StoryPoints
	}
	if body.EstimateHours != nil {
		iss.EstimateHours = body.EstimateHours
	}
	if body.StartDate != nil {
		t, err := parseDatePtr(body.StartDate)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		iss.StartDate = t
	}
	if body.DueDate != nil {
		t, err := parseDatePtr(body.DueDate)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		iss.DueDate = t
	}
	now := platform.Now()
	res, err := s.DB.Exec(`
		UPDATE issues SET title=$1, description=$2, priority=$3, severity=$4, reproduce_steps=$5,
			affect_version=$6, fix_version=$7, assignee_id=$8, sprint_id=$9, story_points=$10,
			estimate_hours=$11, start_date=$12, due_date=$13, version=version+1, updated_at=$14
		WHERE id=$15 AND version=$16`,
		iss.Title, iss.Description, iss.Priority, iss.Severity, iss.ReproduceSteps,
		iss.AffectVersion, iss.FixVersion, iss.AssigneeID, iss.SprintID, iss.StoryPoints,
		iss.EstimateHours, iss.StartDate, iss.DueDate, now, id, iss.Version)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		platform.WriteError(w, r, domain.OptimisticLock())
		return
	}
	if body.Labels != nil {
		_, _ = s.DB.Exec(`DELETE FROM issue_labels WHERE issue_id=$1`, id)
		for _, lb := range body.Labels {
			lb = strings.TrimSpace(lb)
			if lb == "" {
				continue
			}
			_, _ = s.DB.Exec(`INSERT INTO issue_labels (issue_id, label) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, lb)
		}
	}
	out, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteData(w, http.StatusOK, out)
}

func (s *Service) Transition(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var body transitionBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	iss, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if body.Version != 0 && body.Version != iss.Version {
		platform.WriteError(w, r, domain.OptimisticLock())
		return
	}
	role := platform.ProjectRoleFrom(r)
	if _, err := s.Engine.CheckTransition(iss, role, body.To); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var proj domain.Project
	if err := s.DB.Get(&proj, `SELECT * FROM projects WHERE id=$1`, iss.ProjectID); err != nil {
		platform.WriteError(w, r, domain.NotFound("项目不存在"))
		return
	}
	warnings, blocked, err := s.dependencyWarnings(r.Context(), iss)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	if blocked && proj.EnforceDependencyBlock {
		platform.WriteError(w, r, domain.DependencyBlocked(warnings))
		return
	}
	if !blocked {
		warnings = nil
	}
	u := platform.UserFrom(r)
	now := platform.Now()
	from := iss.Status
	var lastChanged time.Time
	_ = s.DB.Get(&lastChanged, `SELECT COALESCE(MAX(changed_at), $2) FROM issue_status_history WHERE issue_id=$1`, id, iss.CreatedAt)
	dur := int(now.Sub(lastChanged).Seconds())
	if dur < 0 {
		dur = 0
	}
	var resolved any
	if domain.IsDoneStatus(body.To) {
		resolved = now
	} else {
		resolved = nil
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(r.Context(), `
		UPDATE issues SET status=$1, version=version+1, updated_at=$2, resolved_at=$3
		WHERE id=$4 AND version=$5`,
		body.To, now, resolved, id, iss.Version)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		platform.WriteError(w, r, domain.OptimisticLock())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO issue_status_history (issue_id, from_status, to_status, actor_id, changed_at, duration_sec)
		VALUES ($1,$2,$3,$4,$5,$6)`, id, from, body.To, u.ID, now, dur); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	col, _ := s.Engine.ColumnOf(iss.IssueType, body.To)
	eventID := NewEventID("issue.status_changed")
	pl := domain.StatusChangedPayload{
		EventID: eventID, IssueID: id, ProjectID: iss.ProjectID, IssueKey: iss.Key,
		Title: iss.Title, IssueType: iss.IssueType, FromStatus: from, ToStatus: body.To,
		ToColumn: col, ActorID: u.ID,
	}
	raw, _ := json.Marshal(pl)
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO outbox_events (event_id, event_type, payload, status, created_at, updated_at)
		VALUES ($1,$2,$3,'pending',$4,$4)`, eventID, domain.EventIssueStatusChanged, raw, now); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	_ = platform.InsertAudit(r.Context(), tx, &u.ID, "issue.transition", "issue", iss.Key, map[string]any{
		"from": from, "to": body.To, "warnings": warnings,
	})
	if err := tx.Commit(); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	out, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteDataWarnings(w, http.StatusOK, out, warnings)
}

func (s *Service) dependencyWarnings(ctx context.Context, iss *domain.Issue) ([]string, bool, error) {
	var keys []string
	err := s.DB.SelectContext(ctx, &keys, `
		SELECT (p.key || '-' || pred.seq_no::text)
		FROM issue_dependencies d
		JOIN issues pred ON pred.id=d.predecessor_id
		JOIN projects p ON p.id=pred.project_id
		WHERE d.successor_id=$1 AND d.dep_type='FS'
		  AND pred.status NOT IN ('DONE','RESOLVED','CLOSED','REJECTED')`, iss.ID)
	if err != nil {
		return nil, false, ctx.Err()
	}
	if len(keys) == 0 {
		return nil, false, nil
	}
	msgs := make([]string, len(keys))
	for i, k := range keys {
		msgs[i] = "前置依赖 " + k + " 尚未完成"
	}
	return msgs, true, nil
}

func (s *Service) History(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var rows []domain.StatusHistory
	err = s.DB.Select(&rows, `
		SELECT h.*, u.display_name AS actor_name
		FROM issue_status_history h JOIN users u ON u.id=h.actor_id
		WHERE h.issue_id=$1 ORDER BY h.changed_at ASC`, id)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (s *Service) Rank(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var body rankBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	rank := MidRank(body.PrevRank, body.NextRank)
	if body.BoardRank != nil {
		rank = *body.BoardRank
	}
	res, err := s.DB.Exec(`UPDATE issues SET board_rank=$1, version=version+1, updated_at=$2 WHERE id=$3 AND version=$4`,
		rank, platform.Now(), id, body.Version)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		platform.WriteError(w, r, domain.OptimisticLock())
		return
	}
	iss, err := s.get(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteData(w, http.StatusOK, iss)
}

func (s *Service) ListDeps(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var rows []domain.Dependency
	err = s.DB.Select(&rows, `
		SELECT d.*,
			(pp.key || '-' || pred.seq_no::text) AS pred_key,
			(sp.key || '-' || succ.seq_no::text) AS succ_key
		FROM issue_dependencies d
		JOIN issues pred ON pred.id=d.predecessor_id
		JOIN issues succ ON succ.id=d.successor_id
		JOIN projects pp ON pp.id=pred.project_id
		JOIN projects sp ON sp.id=succ.project_id
		WHERE d.predecessor_id=$1 OR d.successor_id=$1
		ORDER BY d.id`, id)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (s *Service) AddDep(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var body depBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if body.SuccessorID == 0 {
		body.SuccessorID = id
	}
	if body.PredecessorID == 0 {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "predecessor_id 必填", nil))
		return
	}
	if body.DepType == "" {
		body.DepType = domain.DepFS
	}
	if !domain.ValidDepType(body.DepType) {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法依赖类型", body.DepType))
		return
	}
	var edges []gantt.Edge
	if err := s.DB.Select(&edges, `SELECT predecessor_id AS from_id, successor_id AS to_id FROM issue_dependencies`); err != nil && err != sql.ErrNoRows {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	if cyc, path := gantt.DetectCycle(edges, body.PredecessorID, body.SuccessorID); cyc {
		keys := s.keysFor(r.Context(), path)
		platform.WriteError(w, r, domain.DependencyCycle(keys))
		return
	}
	var depID int64
	err = s.DB.QueryRow(`
		INSERT INTO issue_dependencies (predecessor_id, successor_id, dep_type)
		VALUES ($1,$2,$3) RETURNING id`, body.PredecessorID, body.SuccessorID, body.DepType).Scan(&depID)
	if err != nil {
		if isUnique(err) {
			platform.WriteError(w, r, domain.Conflict(domain.CodeConflict, "依赖已存在", nil))
			return
		}
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusCreated, domain.Dependency{
		ID: depID, PredecessorID: body.PredecessorID, SuccessorID: body.SuccessorID, DepType: body.DepType,
	})
}

func (s *Service) DeleteDep(w http.ResponseWriter, r *http.Request) {
	depID, err := platform.ParseID(chi.URLParam(r, "depID"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_, err = s.DB.Exec(`DELETE FROM issue_dependencies WHERE id=$1`, depID)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) keysFor(ctx context.Context, ids []int64) []string {
	if len(ids) == 0 {
		return nil
	}
	q, args, err := sqlx.In(`
		SELECT i.id, (p.key || '-' || i.seq_no::text) AS k
		FROM issues i JOIN projects p ON p.id=i.project_id WHERE i.id IN (?)`, ids)
	if err != nil {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = itoa(int(id))
		}
		return out
	}
	type row struct {
		ID int64  `db:"id"`
		K  string `db:"k"`
	}
	var rows []row
	_ = s.DB.SelectContext(ctx, &rows, s.DB.Rebind(q), args...)
	m := map[int64]string{}
	for _, r := range rows {
		m[r.ID] = r.K
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		if k, ok := m[id]; ok {
			out[i] = k
		} else {
			out[i] = itoa(int(id))
		}
	}
	return out
}

func NewEventID(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(platform.Now().Format("20060102150405.000000000"), ".", "") + "-" + randHex(6)
}

func randHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	x := uint64(platform.Now().UnixNano())
	for i := 0; i < n; i++ {
		b[i] = hex[x%16]
		x = x/16 + uint64(i)*17
	}
	return string(b)
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique")
}

func itoa(n int) string { return domain.FormatKey("", n)[1:] }
