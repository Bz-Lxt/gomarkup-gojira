package issue_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gojira/internal/issue"
	"gojira/internal/platform"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestListPreservesPaginationTotals(t *testing.T) {
	rawDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("open database stub: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })
	mock.MatchExpectationsInOrder(false)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM issues i WHERE i\.project_id=\$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(37))
	mock.ExpectQuery(`ORDER BY i\.board_rank, i\.id LIMIT \$2 OFFSET \$3`).
		WithArgs(int64(7), int64(20), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	service := &issue.Service{DB: sqlx.NewDb(rawDB, "postgres")}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/7/issues?page=2&per_page=20", nil)
	ctx := context.WithValue(req.Context(), platform.CtxProjectID, int64(7))
	recorder := httptest.NewRecorder()
	service.List(recorder, req.WithContext(ctx))

	if recorder.Code != http.StatusOK {
		t.Fatalf("list returned %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Meta *struct {
			Total      int64 `json:"total"`
			TotalPages int   `json:"total_pages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Meta == nil {
		t.Fatal("list response omitted pagination metadata")
	}
	if response.Meta.Total != 37 || response.Meta.TotalPages != 2 {
		t.Fatalf("pagination metadata = total %d, pages %d; want total 37, pages 2",
			response.Meta.Total, response.Meta.TotalPages)
	}
}
