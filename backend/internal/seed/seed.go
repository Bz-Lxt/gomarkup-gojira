package seed

import (
	"encoding/json"
	"log/slog"
	"time"

	"gojira/internal/auth"
	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/jmoiron/sqlx"
)

type account struct {
	Username, Email, Password, Display, Role string
}

func Run(db *sqlx.DB, log *slog.Logger) error {
	now := platform.Now()
	accounts := []account{
		{"admin", "admin@gojira.local", "Admin@123", "系统管理员", domain.RoleAdmin},
		{"pm", "pm@gojira.local", "Pm@123456", "产品经理", domain.RolePM},
		{"dev", "dev@gojira.local", "Dev@123456", "开发工程师", domain.RoleDev},
		{"qa", "qa@gojira.local", "Qa@123456", "测试工程师", domain.RoleQA},
	}
	ids := map[string]int64{}
	for _, a := range accounts {
		var id int64
		err := db.Get(&id, `SELECT id FROM users WHERE username=$1`, a.Username)
		if err != nil {
			hash, err := auth.HashPassword(a.Password)
			if err != nil {
				return err
			}
			if err := db.QueryRow(`
				INSERT INTO users (username, email, password_hash, display_name, role, is_active, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,TRUE,$6,$6) RETURNING id`,
				a.Username, a.Email, hash, a.Display, a.Role, now).Scan(&id); err != nil {
				return err
			}
		}
		ids[a.Username] = id
	}

	var projectID int64
	err := db.Get(&projectID, `SELECT id FROM projects WHERE key='GJ'`)
	if err != nil {
		if err := db.QueryRow(`
			INSERT INTO projects (key, name, description, owner_id, enforce_dependency_block, workflow_config, created_at, updated_at)
			VALUES ('GJ','GoJira Demo','演示项目：看板 / 燃尽 / 甘特 / 触发器',$1,FALSE,'default',$2,$2)
			RETURNING id`, ids["admin"], now).Scan(&projectID); err != nil {
			return err
		}
	}
	for u, role := range map[string]string{"admin": domain.RoleAdmin, "pm": domain.RolePM, "dev": domain.RoleDev, "qa": domain.RoleQA} {
		_, _ = db.Exec(`
			INSERT INTO project_members (project_id, user_id, role, created_at)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (project_id, user_id) DO UPDATE SET role=EXCLUDED.role`,
			projectID, ids[u], role, now)
	}

	monday := mondayOf(now)
	end := monday.AddDate(0, 0, 13)
	var sprintID int64
	err = db.Get(&sprintID, `SELECT id FROM sprints WHERE project_id=$1 AND name='Sprint 1'`, projectID)
	if err != nil {
		if err := db.QueryRow(`
			INSERT INTO sprints (project_id, name, goal, start_date, end_date, status, committed_points, created_at, updated_at)
			VALUES ($1,'Sprint 1','打通看板状态机与燃尽演示',$2,$3,'ACTIVE',34,$4,$4)
			RETURNING id`, projectID, monday, end, now).Scan(&sprintID); err != nil {
			return err
		}
	}

	type hist struct {
		From, To string
		DayOff   int
		Hour     int
		Actor    string
	}
	type spec struct {
		Seq, Points                   int
		Type, Title, Status, Priority string
		Severity                      *string
		Assignee                      string
		StartOff, DueOff              int
		CreatedOff                    int
		History                       []hist
	}
	sev := func(s string) *string { return &s }
	issues := []spec{
		{1, 8, domain.TypeStory, "用户登录与 JWT 刷新", "DONE", "HIGH", nil, "dev", 0, 3, 0, []hist{
			{"TODO", "IN_PROGRESS", 0, 10, "dev"},
			{"IN_PROGRESS", "TESTING", 1, 16, "dev"},
			{"TESTING", "DONE", 2, 11, "qa"},
		}},
		{2, 5, domain.TypeStory, "项目与成员管理", "IN_PROGRESS", "MEDIUM", nil, "dev", 1, 8, 0, []hist{
			{"TODO", "IN_PROGRESS", 1, 9, "dev"},
		}},
		{3, 3, domain.TypeTask, "看板四列接口", "TODO", "HIGH", nil, "dev", 2, 9, 0, nil},
		{4, 3, domain.TypeTask, "Issue 抽屉表单", "TESTING", "MEDIUM", nil, "dev", 1, 6, 0, []hist{
			{"TODO", "IN_PROGRESS", 2, 9, "dev"},
			{"IN_PROGRESS", "TESTING", 3, 14, "dev"},
		}},
		{5, 2, domain.TypeTask, "健康检查与日志", "DONE", "LOW", nil, "dev", 0, 2, 0, []hist{
			{"TODO", "IN_PROGRESS", 0, 11, "dev"},
			{"IN_PROGRESS", "TESTING", 0, 17, "dev"},
			{"TESTING", "DONE", 1, 10, "qa"},
		}},
		{6, 5, domain.TypeBug, "FIXED 状态的 Bug 无法被 QA 验证", "FIXED", "HIGHEST", sev("MAJOR"), "dev", 2, 7, 1, []hist{
			{"NEW", "CONFIRMED", 1, 10, "qa"},
			{"CONFIRMED", "FIXING", 2, 11, "dev"},
			{"FIXING", "FIXED", 3, 15, "dev"},
		}},
		{7, 3, domain.TypeBug, "邮件触发器偶发漏发", "NEW", "HIGH", sev("CRITICAL"), "qa", 3, 10, 2, nil},
		{8, 2, domain.TypeBug, "甘特条日期错位一天", "FIXING", "MEDIUM", sev("MINOR"), "dev", 2, 8, 1, []hist{
			{"NEW", "CONFIRMED", 2, 9, "qa"},
			{"CONFIRMED", "FIXING", 3, 10, "dev"},
		}},
		{9, 5, domain.TypeStory, "依赖环检测", "TODO", "HIGH", nil, "dev", 4, 11, 0, nil},
		{10, 3, domain.TypeTask, "Sprint 启停", "IN_PROGRESS", "MEDIUM", nil, "pm", 3, 10, 1, []hist{
			{"TODO", "IN_PROGRESS", 3, 9, "dev"},
		}},
		{11, 5, domain.TypeStory, "燃尽图聚合 SQL", "DONE", "HIGH", nil, "dev", 0, 4, 0, []hist{
			{"TODO", "IN_PROGRESS", 0, 14, "dev"},
			{"IN_PROGRESS", "TESTING", 2, 11, "dev"},
			{"TESTING", "DONE", 3, 16, "qa"},
		}},
		{12, 2, domain.TypeTask, "种子数据与演示账号", "DONE", "LOW", nil, "admin", 0, 1, 0, []hist{
			{"TODO", "IN_PROGRESS", 0, 9, "admin"},
			{"IN_PROGRESS", "TESTING", 0, 15, "admin"},
			{"TESTING", "DONE", 1, 9, "qa"},
		}},
		{13, 1, domain.TypeTask, "评论 @提及", "TODO", "LOW", nil, "", 5, 12, 4, nil},
	}

	var existing int
	_ = db.Get(&existing, `SELECT COUNT(*) FROM issues WHERE project_id=$1`, projectID)
	if existing == 0 {
		for _, spec := range issues {
			var assignee *int64
			if spec.Assignee != "" {
				id := ids[spec.Assignee]
				assignee = &id
			}
			created := monday.AddDate(0, 0, spec.CreatedOff).Add(9 * time.Hour)
			start := monday.AddDate(0, 0, spec.StartOff)
			due := monday.AddDate(0, 0, spec.DueOff)
			rank := float64(spec.Seq) * 1000
			var resolved *time.Time
			if domain.IsDoneStatus(spec.Status) && len(spec.History) > 0 {
				last := spec.History[len(spec.History)-1]
				t := monday.AddDate(0, 0, last.DayOff).Add(time.Duration(last.Hour) * time.Hour)
				resolved = &t
			}
			var iid int64
			if err := db.QueryRow(`
				INSERT INTO issues (
					project_id, seq_no, issue_type, title, description, status, priority, severity,
					assignee_id, reporter_id, sprint_id, story_points, estimate_hours,
					start_date, due_date, board_rank, version, created_at, updated_at, resolved_at
				) VALUES (
					$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,1,$17,$17,$18
				) RETURNING id`,
				projectID, spec.Seq, spec.Type, spec.Title, spec.Title+" — 演示描述", spec.Status, spec.Priority, spec.Severity,
				assignee, ids["pm"], sprintID, spec.Points, float64(spec.Points)*1.5,
				start, due, rank, created, resolved,
			).Scan(&iid); err != nil {
				return err
			}
			_, _ = db.Exec(`INSERT INTO issue_labels (issue_id, label) VALUES ($1,$2) ON CONFLICT DO NOTHING`, iid, spec.Type)
			prev := created
			from := initialOf(spec.Type)
			for _, h := range spec.History {
				at := monday.AddDate(0, 0, h.DayOff).Add(time.Duration(h.Hour) * time.Hour)
				if h.From != "" {
					from = h.From
				}
				dur := int(at.Sub(prev).Seconds())
				if dur < 0 {
					dur = 0
				}
				_, _ = db.Exec(`
					INSERT INTO issue_status_history (issue_id, from_status, to_status, actor_id, changed_at, duration_sec)
					VALUES ($1,$2,$3,$4,$5,$6)`, iid, from, h.To, ids[h.Actor], at, dur)
				from = h.To
				prev = at
			}
		}
		var pred, succ int64
		_ = db.Get(&pred, `SELECT id FROM issues WHERE project_id=$1 AND seq_no=3`, projectID)
		_ = db.Get(&succ, `SELECT id FROM issues WHERE project_id=$1 AND seq_no=9`, projectID)
		if pred > 0 && succ > 0 {
			_, _ = db.Exec(`
				INSERT INTO issue_dependencies (predecessor_id, successor_id, dep_type)
				VALUES ($1,$2,'FS') ON CONFLICT DO NOTHING`, pred, succ)
		}
	}

	var trig int
	_ = db.Get(&trig, `SELECT COUNT(*) FROM triggers WHERE project_id=$1 AND name='完成列通知 PM'`, projectID)
	if trig == 0 {
		cond, _ := json.Marshal(map[string]any{"to_column": "DONE"})
		acts, _ := json.Marshal([]map[string]any{{"type": domain.ActionSendEmail, "to_role": domain.RolePM}})
		_, err := db.Exec(`
			INSERT INTO triggers (project_id, name, event_type, condition, actions, is_enabled, created_at)
			VALUES ($1,$2,$3,$4,$5,TRUE,$6)`,
			projectID, "完成列通知 PM", domain.EventIssueStatusChanged, cond, acts, now)
		if err != nil {
			return err
		}
	}

	log.Info("seed complete", "project", "GJ", "sprint", sprintID, "users", 4)
	return nil
}

func initialOf(t string) string {
	if t == domain.TypeBug {
		return "NEW"
	}
	return "TODO"
}

func mondayOf(t time.Time) time.Time {
	t = platform.StartOfDay(t)
	off := int(t.Weekday() - time.Monday)
	if off < 0 {
		off += 7
	}
	return t.AddDate(0, 0, -off)
}
