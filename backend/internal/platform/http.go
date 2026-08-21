package platform

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"gojira/internal/domain"
)

type ctxKey int

const (
	CtxRequestID ctxKey = iota
	CtxUser
	CtxProjectID
	CtxProjectRole
)

type UserPrincipal struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

type PageMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type Envelope struct {
	Data     any       `json:"data"`
	Meta     *PageMeta `json:"meta,omitempty"`
	Warnings []string  `json:"warnings,omitempty"`
}

func RequestIDFrom(r *http.Request) string {
	if v, ok := r.Context().Value(CtxRequestID).(string); ok {
		return v
	}
	return r.Header.Get("X-Request-ID")
}

func UserFrom(r *http.Request) *UserPrincipal {
	v, _ := r.Context().Value(CtxUser).(*UserPrincipal)
	return v
}

func ProjectRoleFrom(r *http.Request) string {
	v, _ := r.Context().Value(CtxProjectRole).(string)
	return v
}

func ProjectIDFrom(r *http.Request) int64 {
	v, _ := r.Context().Value(CtxProjectID).(int64)
	return v
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func WriteData(w http.ResponseWriter, status int, data any) {
	WriteJSON(w, status, Envelope{Data: data})
}

func WriteDataMeta(w http.ResponseWriter, status int, data any, meta *PageMeta) {
	WriteJSON(w, status, Envelope{Data: data, Meta: meta})
}

func WriteDataWarnings(w http.ResponseWriter, status int, data any, warnings []string) {
	WriteJSON(w, status, Envelope{Data: data, Warnings: warnings})
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	rid := RequestIDFrom(r)
	var ae *domain.AppError
	if errors.As(err, &ae) {
		WriteJSON(w, ae.HTTPStatus, domain.ErrorBody{
			Code:      ae.Code,
			Message:   ae.Message,
			Details:   ae.Details,
			RequestID: rid,
		})
		return
	}
	slog.Error("unhandled error", "err", err, "request_id", rid, "path", r.URL.Path)
	WriteJSON(w, http.StatusInternalServerError, domain.ErrorBody{
		Code:      domain.CodeInternal,
		Message:   "内部错误",
		Details:   nil,
		RequestID: rid,
	})
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return domain.BadRequest("INVALID_JSON", "请求体不是合法 JSON", err.Error())
	}
	return nil
}

func QueryInt(r *http.Request, key string, fallback int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func ParsePage(r *http.Request) (page, perPage, offset int) {
	page = QueryInt(r, "page", 1)
	perPage = QueryInt(r, "per_page", 20)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset = (page - 1) * perPage
	return page, perPage, offset
}

func NewMeta(page, perPage int, total int64) *PageMeta {
	pages := int(total) / perPage
	if int(total)%perPage != 0 {
		pages++
	}
	return &PageMeta{Page: page, PerPage: perPage, Total: total, TotalPages: pages}
}

func ParseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id < 1 {
		return 0, domain.BadRequest("INVALID_ID", "非法资源编号", s)
	}
	return id, nil
}
