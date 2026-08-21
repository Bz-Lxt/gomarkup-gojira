package issue_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gojira/internal/domain"
	"gojira/internal/issue"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func TestAddDependencyRejectsCycleThroughConvergingBranches(t *testing.T) {
	rawDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	db := sqlx.NewDb(rawDB, "postgres")
	mock.ExpectQuery(`SELECT predecessor_id AS from_id, successor_id AS to_id FROM issue_dependencies`).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id"}).
			AddRow(1, 2).
			AddRow(1, 3).
			AddRow(2, 4).
			AddRow(3, 4).
			AddRow(4, 5))
	mock.ExpectQuery(`SELECT i\.id|INSERT INTO issue_dependencies`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))

	svc := &issue.Service{DB: db}
	router := chi.NewRouter()
	router.Post("/issues/{id}/dependencies", svc.AddDep)

	req := httptest.NewRequest(http.MethodPost, "/issues/1/dependencies",
		strings.NewReader(`{"predecessor_id":5,"dep_type":"FS"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("adding a closing dependency returned %d, want %d: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != domain.CodeDependencyCycle {
		t.Fatalf("error code = %q, want %q", body.Code, domain.CodeDependencyCycle)
	}
}
