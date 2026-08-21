package stats_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gojira/internal/platform"
	"gojira/internal/stats"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func TestBurndownScopeChangesDoNotOverwriteActual(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	day := time.Date(2026, 8, 24, 0, 0, 0, 0, platform.Location())
	mock.ExpectQuery(".*").
		WithArgs(int64(12), "points").
		WillReturnRows(sqlmock.NewRows([]string{"day", "remaining", "scope", "committed"}).
			AddRow(day, 12.0, 12.0, 12.0).
			AddRow(day.Add(24*time.Hour), 17.0, 17.0, 12.0).
			AddRow(day.Add(48*time.Hour), 9.0, 17.0, 12.0))

	service := &stats.Service{DB: sqlx.NewDb(db, "postgres")}
	router := chi.NewRouter()
	router.Get("/sprints/{id}/burndown", service.Burndown)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/sprints/12/burndown?metric=points", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET burndown returned %d: %s", recorder.Code, recorder.Body.String())
	}

	type point struct {
		Date  string  `json:"date"`
		Value float64 `json:"value"`
	}
	type scopeChange struct {
		Date  string  `json:"date"`
		Delta float64 `json:"delta"`
	}
	var response struct {
		Data struct {
			Ideal        []point       `json:"ideal"`
			Actual       []point       `json:"actual"`
			ScopeChanges []scopeChange `json:"scope_changes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wantActual := []point{
		{Date: "2026-08-24", Value: 12},
		{Date: "2026-08-25", Value: 17},
		{Date: "2026-08-26", Value: 9},
	}
	if len(response.Data.Actual) != len(wantActual) {
		t.Fatalf("actual has %d points, want %d: %+v", len(response.Data.Actual), len(wantActual), response.Data.Actual)
	}
	for i, want := range wantActual {
		if got := response.Data.Actual[i]; got != want {
			t.Errorf("actual[%d] = %+v, want %+v", i, got, want)
		}
	}

	wantIdeal := []point{
		{Date: "2026-08-24", Value: 12},
		{Date: "2026-08-25", Value: 6},
		{Date: "2026-08-26", Value: 0},
	}
	if len(response.Data.Ideal) != len(wantIdeal) {
		t.Fatalf("ideal has %d points, want %d: %+v", len(response.Data.Ideal), len(wantIdeal), response.Data.Ideal)
	}
	for i, want := range wantIdeal {
		if got := response.Data.Ideal[i]; got != want {
			t.Errorf("ideal[%d] = %+v, want %+v", i, got, want)
		}
	}

	if len(response.Data.ScopeChanges) != 1 {
		t.Fatalf("scope_changes = %+v, want one entry", response.Data.ScopeChanges)
	}
	wantChange := scopeChange{Date: "2026-08-25", Delta: 5}
	if got := response.Data.ScopeChanges[0]; got != wantChange {
		t.Errorf("scope_changes[0] = %+v, want %+v", got, wantChange)
	}
}
