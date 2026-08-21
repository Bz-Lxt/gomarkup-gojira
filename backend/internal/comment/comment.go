package comment

import (
	"net/http"
	"regexp"
	"strings"

	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

var mentionRe = regexp.MustCompile(`@([A-Za-z0-9_]{2,32})`)

type Service struct {
	DB *sqlx.DB
}

type createBody struct {
	Body string `json:"body"`
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var rows []domain.Comment
	err = s.DB.Select(&rows, `
		SELECT c.*, u.display_name AS author_name
		FROM issue_comments c JOIN users u ON u.id=c.author_id
		WHERE c.issue_id=$1 ORDER BY c.created_at ASC`, id)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, rows)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	id, err := platform.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var body createBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "评论不能为空", nil))
		return
	}
	if len(body.Body) > 8000 {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "评论过长", nil))
		return
	}
	u := platform.UserFrom(r)
	now := platform.Now()
	var c domain.Comment
	err = s.DB.QueryRowx(`
		INSERT INTO issue_comments (issue_id, author_id, body, created_at)
		VALUES ($1,$2,$3,$4) RETURNING id, issue_id, author_id, body, created_at`,
		id, u.ID, body.Body, now,
	).StructScan(&c)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	c.Author = u.DisplayName
	for _, m := range mentionRe.FindAllStringSubmatch(body.Body, -1) {
		var uid int64
		if err := s.DB.Get(&uid, `SELECT id FROM users WHERE username=$1`, m[1]); err != nil {
			continue
		}
		_, _ = s.DB.Exec(`
			INSERT INTO notifications (user_id, type, title, body, is_read, created_at)
			VALUES ($1,'mention',$2,$3,FALSE,$4)`,
			uid, "有人在评论中提到你", body.Body, now)
	}
	platform.WriteData(w, http.StatusCreated, c)
}

func Mentions(body string) []string {
	ms := mentionRe.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(ms))
	seen := map[string]struct{}{}
	for _, m := range ms {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	return out
}
