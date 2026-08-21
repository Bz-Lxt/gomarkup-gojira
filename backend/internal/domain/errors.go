package domain

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	CodeInvalidJSON       = "INVALID_JSON"
	CodeInvalidInput      = "INVALID_INPUT"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeNotFound          = "NOT_FOUND"
	CodeConflict          = "CONFLICT"
	CodeInvalidTransition = "INVALID_TRANSITION"
	CodeGuardFailed       = "GUARD_FAILED"
	CodeOptimisticLock    = "OPTIMISTIC_LOCK"
	CodeDependencyCycle   = "DEPENDENCY_CYCLE"
	CodeDependencyBlocked = "DEPENDENCY_BLOCKED"
	CodeWeakPassword      = "WEAK_PASSWORD"
	CodeInternal          = "INTERNAL"
	CodeUnprocessable     = "UNPROCESSABLE"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrWeakPassword = errors.New("weak password")
	ErrGuardFailed  = errors.New("guard failed")
	ErrOptimistic   = errors.New("optimistic lock conflict")
)

// ErrorBody is the public error envelope.
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details"`
	RequestID string `json:"request_id"`
}

// AppError is the typed domain error mapped to HTTP.
type AppError struct {
	Code       string
	Message    string
	Details    any
	HTTPStatus int
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func newErr(status int, code, msg string, details any, err error) *AppError {
	return &AppError{Code: code, Message: msg, Details: details, HTTPStatus: status, Err: err}
}

func BadRequest(code, msg string, details any) *AppError {
	return newErr(http.StatusBadRequest, code, msg, details, nil)
}

func Unauthorized(msg string) *AppError {
	return newErr(http.StatusUnauthorized, CodeUnauthorized, msg, nil, ErrUnauthorized)
}

func Forbidden(msg string, details any) *AppError {
	return newErr(http.StatusForbidden, CodeForbidden, msg, details, ErrForbidden)
}

func NotFound(msg string) *AppError {
	return newErr(http.StatusNotFound, CodeNotFound, msg, nil, ErrNotFound)
}

func Conflict(code, msg string, details any) *AppError {
	return newErr(http.StatusConflict, code, msg, details, ErrConflict)
}

func Unprocessable(code, msg string, details any) *AppError {
	return newErr(http.StatusUnprocessableEntity, code, msg, details, nil)
}

func InvalidTransition(legal []string) *AppError {
	return Unprocessable(CodeInvalidTransition, "非法状态转换", map[string]any{
		"legal_successors": legal,
	})
}

func GuardFailed(guard, reason string) *AppError {
	return Unprocessable(CodeGuardFailed, reason, map[string]any{"guard": guard})
}

func OptimisticLock() *AppError {
	return Conflict(CodeOptimisticLock, "资源已被他人更新，请刷新后重试", nil)
}

func DependencyCycle(path []string) *AppError {
	return Conflict(CodeDependencyCycle, "依赖关系存在环", map[string]any{"cycle": path})
}

func DependencyBlocked(keys []string) *AppError {
	return Conflict(CodeDependencyBlocked, "前置依赖尚未完成", map[string]any{"predecessors": keys})
}

func WeakPassword(reason string) *AppError {
	return Unprocessable(CodeWeakPassword, reason, nil)
}

func Internal(err error) *AppError {
	return newErr(http.StatusInternalServerError, CodeInternal, "内部错误", nil, err)
}
