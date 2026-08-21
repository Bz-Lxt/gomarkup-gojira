package project

import (
	"net/http"
	"regexp"
	"strings"

	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

type Service struct {
	DB *sqlx.DB
}

type createBody struct {
	Key                    string `json:"key"`
	Name                   string `json:"name"`
	Description            string `json:"description"`
	EnforceDependencyBlock *bool  `json:"enforce_dependency_block"`
}

type patchBody struct {
	Name                   *string `json:"name"`
	Description            *string `json:"description"`
	EnforceDependencyBlock *bool   `json:"enforce_dependency_block"`
}

type memberBody struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	u := platform.UserFrom(r)
	page, per, offset := platform.ParsePage(r)
	var total int64
	var rows []domain.Project
	if u.Role == domain.RoleAdmin {
		_ = s.DB.Get(&total, `SELECT COUNT(*) FROM projects`)
		err := s.DB.Select(&rows, `SELECT * FROM projects ORDER BY id LIMIT $1 OFFSET $2`, per, offset)
		if err != nil {
			platform.WriteError(w, r, domain.Internal(err))
			return
		}
	} else {
		_ = s.DB.Get(&total, `SELECT COUNT(*) FROM projects p JOIN project_members m ON m.project_id=p.id WHERE m.user_id=$1`, u.ID)
		err := s.DB.Select(&rows, `
			SELECT p.* FROM projects p
			JOIN project_members m ON m.project_id=p.id
			WHERE m.user_id=$1
			ORDER BY p.id LIMIT $2 OFFSET $3`, u.ID, per, offset)
		if err != nil {
			platform.WriteError(w, r, domain.Internal(err))
			return
		}
	}
	platform.WriteDataMeta(w, http.StatusOK, rows, platform.NewMeta(page, per, total))
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	u := platform.UserFrom(r)
	if u.Role != domain.RoleAdmin && u.Role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("仅管理员或产品经理可创建项目", nil))
		return
	}
	var body createBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	body.Key = strings.ToUpper(strings.TrimSpace(body.Key))
	body.Name = strings.TrimSpace(body.Name)
	if !keyRe.MatchString(body.Key) {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "项目 Key 须为 2-10 位大写字母数字且以字母开头", body.Key))
		return
	}
	if body.Name == "" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "项目名称必填", nil))
		return
	}
	enforce := false
	if body.EnforceDependencyBlock != nil {
		enforce = *body.EnforceDependencyBlock
	}
	now := platform.Now()
	tx, err := s.DB.Beginx()
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	defer func() { _ = tx.Rollback() }()
	var p domain.Project
	err = tx.QueryRowx(`
		INSERT INTO projects (key, name, description, owner_id, enforce_dependency_block, workflow_config, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'default',$6,$6)
		RETURNING id, key, name, description, owner_id, enforce_dependency_block, workflow_config, created_at, updated_at`,
		body.Key, body.Name, body.Description, u.ID, enforce, now,
	).StructScan(&p)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			platform.WriteError(w, r, domain.Conflict(domain.CodeConflict, "项目 Key 已存在", body.Key))
			return
		}
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	_, err = tx.Exec(`INSERT INTO project_members (project_id, user_id, role, created_at) VALUES ($1,$2,$3,$4)`,
		p.ID, u.ID, u.Role, now)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	_ = platform.InsertAudit(r.Context(), tx, &u.ID, "project.create", "project", domain.FormatKey(p.Key, int(p.ID)), p)
	if err := tx.Commit(); err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+itoa(p.ID))
	platform.WriteData(w, http.StatusCreated, p)
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	param := strings.TrimSpace(chi.URLParam(r, "id"))
	var p domain.Project
	if id, err := platform.ParseID(param); err == nil {
		if err := s.DB.Get(&p, `SELECT * FROM projects WHERE id=$1`, id); err != nil {
			platform.WriteError(w, r, domain.NotFound("项目不存在"))
			return
		}
	} else if err := s.DB.Get(&p, `SELECT * FROM projects WHERE key=$1`, strings.ToUpper(param)); err != nil {
		platform.WriteError(w, r, domain.NotFound("项目不存在"))
		return
	}
	platform.WriteData(w, http.StatusOK, p)
}

func (s *Service) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	role := platform.ProjectRoleFrom(r)
	if role != domain.RoleAdmin && role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("仅项目管理员或产品经理可更新项目", nil))
		return
	}
	var body patchBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var p domain.Project
	if err := s.DB.Get(&p, `SELECT * FROM projects WHERE id=$1`, id); err != nil {
		platform.WriteError(w, r, domain.NotFound("项目不存在"))
		return
	}
	if body.Name != nil {
		p.Name = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.EnforceDependencyBlock != nil {
		p.EnforceDependencyBlock = *body.EnforceDependencyBlock
	}
	p.UpdatedAt = platform.Now()
	_, err = s.DB.Exec(`UPDATE projects SET name=$1, description=$2, enforce_dependency_block=$3, updated_at=$4 WHERE id=$5`,
		p.Name, p.Description, p.EnforceDependencyBlock, p.UpdatedAt, id)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, p)
}

func (s *Service) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := platform.ProjectIDFrom(r)
	var rows []domain.ProjectMember
	err := s.DB.Select(&rows, `
		SELECT m.project_id, m.user_id, m.role, m.created_at, u.username, u.email, u.display_name
		FROM project_members m JOIN users u ON u.id=m.user_id
		WHERE m.project_id=$1 ORDER BY m.user_id`, id)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (s *Service) AddMember(w http.ResponseWriter, r *http.Request) {
	if role := platform.ProjectRoleFrom(r); role != domain.RoleAdmin && role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("无权管理成员", nil))
		return
	}
	var body memberBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if !domain.ValidRole(body.Role) {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "非法角色", body.Role))
		return
	}
	now := platform.Now()
	_, err := s.DB.Exec(`
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role=EXCLUDED.role`,
		platform.ProjectIDFrom(r), body.UserID, body.Role, now)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusCreated, body)
}

func (s *Service) RemoveMember(w http.ResponseWriter, r *http.Request) {
	if role := platform.ProjectRoleFrom(r); role != domain.RoleAdmin && role != domain.RolePM {
		platform.WriteError(w, r, domain.Forbidden("无权管理成员", nil))
		return
	}
	uid, err := platform.ParseID(chi.URLParam(r, "userID"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_, err = s.DB.Exec(`DELETE FROM project_members WHERE project_id=$1 AND user_id=$2`,
		platform.ProjectIDFrom(r), uid)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func itoa(n int64) string {
	return domain.FormatKey("", int(n))[1:]
}
