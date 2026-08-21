package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gojira/internal/domain"
)

func loadRepoEngine(t *testing.T) *Engine {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "config", "workflow.yaml"),
		filepath.Join("..", "..", "..", "..", "config", "workflow.yaml"),
		"/app/config/workflow.yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			e, err := Load(p)
			if err != nil {
				t.Fatalf("Load(%s): %v", p, err)
			}
			return e
		}
	}
	e, err := Parse([]byte(minimalYAML))
	if err != nil {
		t.Fatalf("Parse fallback: %v", err)
	}
	return e
}

const minimalYAML = `
version: 1
roles: [ADMIN, PM, DEV, QA, VIEWER]
columns:
  - {id: TODO, label: 待处理}
  - {id: IN_PROGRESS, label: 开发中}
  - {id: TESTING, label: 已测试}
  - {id: DONE, label: 已完成}
workflows:
  task_workflow:
    applies_to: [STORY, TASK]
    initial: TODO
    states:
      TODO: {column: TODO, terminal: false}
      IN_PROGRESS: {column: IN_PROGRESS, terminal: false}
      TESTING: {column: TESTING, terminal: false}
      DONE: {column: DONE, terminal: true}
    transitions:
      - {from: TODO, to: IN_PROGRESS, allowed_roles: [DEV, PM, ADMIN], guards: [has_assignee]}
      - {from: IN_PROGRESS, to: TESTING, allowed_roles: [DEV, QA, ADMIN]}
      - {from: TESTING, to: DONE, allowed_roles: [QA, ADMIN]}
      - {from: IN_PROGRESS, to: TODO, allowed_roles: [PM, ADMIN]}
      - {from: TESTING, to: IN_PROGRESS, allowed_roles: [QA, ADMIN]}
      - {from: DONE, to: TESTING, allowed_roles: [PM, ADMIN]}
  bug_workflow:
    applies_to: [BUG]
    initial: NEW
    states:
      NEW: {column: TODO, terminal: false}
      CONFIRMED: {column: TODO, terminal: false}
      FIXING: {column: IN_PROGRESS, terminal: false}
      FIXED: {column: TESTING, terminal: false}
      RESOLVED: {column: DONE, terminal: false}
      CLOSED: {column: DONE, terminal: true}
      REJECTED: {column: DONE, terminal: true}
      REOPENED: {column: IN_PROGRESS, terminal: false}
    transitions:
      - {from: NEW, to: CONFIRMED, allowed_roles: [QA, PM, ADMIN]}
      - {from: NEW, to: REJECTED, allowed_roles: [QA, PM]}
      - {from: CONFIRMED, to: FIXING, allowed_roles: [DEV, ADMIN], guards: [has_assignee]}
      - {from: FIXING, to: FIXED, allowed_roles: [DEV, ADMIN]}
      - {from: FIXED, to: RESOLVED, allowed_roles: [QA]}
      - {from: FIXED, to: FIXING, allowed_roles: [QA]}
      - {from: RESOLVED, to: CLOSED, allowed_roles: [PM, QA, ADMIN]}
      - {from: CLOSED, to: REOPENED, allowed_roles: [QA, PM, ADMIN]}
      - {from: REJECTED, to: REOPENED, allowed_roles: [QA, PM, ADMIN]}
      - {from: REOPENED, to: FIXING, allowed_roles: [DEV, ADMIN], guards: [has_assignee]}
      - {from: REOPENED, to: FIXED, allowed_roles: [DEV, ADMIN]}
`

func assignee(id int64) *int64 { return &id }

func TestValidTaskTransition(t *testing.T) {
	e := loadRepoEngine(t)
	issue := &domain.Issue{IssueType: domain.TypeTask, Status: "TODO", AssigneeID: assignee(1)}
	if _, err := e.CheckTransition(issue, domain.RoleDev, "IN_PROGRESS"); err != nil {
		t.Fatalf("expected legal transition: %v", err)
	}
}

func TestIllegalEdgeListsSuccessors(t *testing.T) {
	e := loadRepoEngine(t)
	issue := &domain.Issue{IssueType: domain.TypeStory, Status: "TODO", AssigneeID: assignee(1)}
	_, err := e.CheckTransition(issue, domain.RoleDev, "DONE")
	if err == nil {
		t.Fatal("expected 422")
	}
	var ae *domain.AppError
	if !errors.As(err, &ae) || ae.HTTPStatus != 422 {
		t.Fatalf("want 422 AppError, got %v", err)
	}
	details, _ := ae.Details.(map[string]any)
	legal, _ := details["legal_successors"].([]string)
	if len(legal) == 0 {
		t.Fatalf("legal_successors should be listed: %+v", ae.Details)
	}
	found := false
	for _, s := range legal {
		if s == "IN_PROGRESS" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IN_PROGRESS in %v", legal)
	}
}

func TestDevCannotResolveBug(t *testing.T) {
	e := loadRepoEngine(t)
	issue := &domain.Issue{IssueType: domain.TypeBug, Status: "FIXED", AssigneeID: assignee(2)}
	tests := []struct {
		name string
		role string
		ok   bool
	}{
		{"dev", domain.RoleDev, false},
		{"admin", domain.RoleAdmin, false},
		{"pm", domain.RolePM, false},
		{"qa", domain.RoleQA, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.CheckTransition(issue, tt.role, "RESOLVED")
			if tt.ok && err != nil {
				t.Fatalf("QA should resolve: %v", err)
			}
			if !tt.ok {
				var ae *domain.AppError
				if err == nil || !errors.As(err, &ae) || ae.HTTPStatus != 403 {
					t.Fatalf("role %s should get 403, got %v", tt.role, err)
				}
			}
		})
	}
}

func TestQACanResolveBug(t *testing.T) {
	e := loadRepoEngine(t)
	issue := &domain.Issue{IssueType: domain.TypeBug, Status: "FIXED"}
	if _, err := e.CheckTransition(issue, domain.RoleQA, "RESOLVED"); err != nil {
		t.Fatalf("QA FIXED→RESOLVED: %v", err)
	}
}

func TestHasAssigneeGuard(t *testing.T) {
	e := loadRepoEngine(t)
	issue := &domain.Issue{IssueType: domain.TypeTask, Status: "TODO"}
	_, err := e.CheckTransition(issue, domain.RoleDev, "IN_PROGRESS")
	if err == nil {
		t.Fatal("expected guard failure")
	}
	var ae *domain.AppError
	if !errors.As(err, &ae) || ae.HTTPStatus != 422 || ae.Code != domain.CodeGuardFailed {
		t.Fatalf("want 422 GUARD_FAILED, got %v", err)
	}
}

func TestValidateRejectsUnknownGuard(t *testing.T) {
	raw := []byte(`
version: 1
roles: [DEV]
columns: [{id: TODO, label: x}, {id: DONE, label: y}]
workflows:
  t:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: TODO}
      DONE: {column: DONE, terminal: true}
    transitions:
      - {from: TODO, to: DONE, allowed_roles: [DEV], guards: [not_a_real_guard]}
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected unknown guard to fail validation")
	}
}

func TestValidateRejectsOrphanState(t *testing.T) {
	raw := []byte(`
version: 1
roles: [DEV]
columns: [{id: TODO, label: x}, {id: DONE, label: y}]
workflows:
  t:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: TODO}
      DONE: {column: DONE, terminal: true}
      GHOST: {column: TODO}
    transitions:
      - {from: TODO, to: DONE, allowed_roles: [DEV]}
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected orphan GHOST to fail")
	}
}

func TestColumnMapping(t *testing.T) {
	e := loadRepoEngine(t)
	tests := []struct {
		typ, status, col string
	}{
		{domain.TypeTask, "TODO", "TODO"},
		{domain.TypeTask, "DONE", "DONE"},
		{domain.TypeBug, "NEW", "TODO"},
		{domain.TypeBug, "FIXED", "TESTING"},
		{domain.TypeBug, "RESOLVED", "DONE"},
		{domain.TypeBug, "REJECTED", "DONE"},
		{domain.TypeBug, "REOPENED", "IN_PROGRESS"},
	}
	for _, tt := range tests {
		col, ok := e.ColumnOf(tt.typ, tt.status)
		if !ok || col != tt.col {
			t.Errorf("%s %s: got %s/%v want %s", tt.typ, tt.status, col, ok, tt.col)
		}
	}
}

func TestEngineHelpers(t *testing.T) {
	e := loadRepoEngine(t)
	st, err := e.InitialStatus(domain.TypeTask)
	if err != nil || st != "TODO" {
		t.Fatalf("initial task: %s %v", st, err)
	}
	st, err = e.InitialStatus(domain.TypeBug)
	if err != nil || st != "NEW" {
		t.Fatalf("initial bug: %s %v", st, err)
	}
	if _, err := e.InitialStatus("EPIC"); err == nil {
		t.Fatal("unknown type")
	}
	if !e.IsColumn(domain.TypeBug, "FIXED", "TESTING") {
		t.Fatal("FIXED maps to TESTING")
	}
	if _, ok := e.ColumnMeta("DONE"); !ok {
		t.Fatal("DONE column")
	}
	if len(e.Columns()) < 4 {
		t.Fatal("four board columns")
	}
	if len(GuardNames()) < 2 {
		t.Fatal("guards registered")
	}
	tmp := filepath.Join(t.TempDir(), "wf.yaml")
	if err := os.WriteFile(tmp, []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing file")
	}
}

func TestValidateMoreFailures(t *testing.T) {
	cases := []string{
		`version: 0`,
		`version: 1
roles: []
columns: [{id: TODO, label: x}]
workflows: {}`,
		`version: 1
roles: [DEV]
columns: [{id: TODO, label: x}]
workflows:
  t:
    applies_to: [TASK]
    initial: MISSING
    states: {TODO: {column: TODO, terminal: true}}
    transitions: []`,
		`version: 1
roles: [DEV]
columns: [{id: TODO, label: x}]
workflows:
  t:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: TODO, terminal: false}
    transitions:
      - {from: TODO, to: TODO, allowed_roles: [DEV]}`,
		`version: 1
roles: [DEV]
columns: [{id: TODO, label: x}]
workflows:
  t:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: GHOST}
      DONE: {column: TODO, terminal: true}
    transitions:
      - {from: TODO, to: DONE, allowed_roles: [DEV]}`,
		`version: 1
roles: [DEV]
columns: [{id: TODO, label: x}]
workflows:
  t:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: TODO}
      DONE: {column: TODO, terminal: true}
    transitions:
      - {from: TODO, to: DONE, allowed_roles: [GHOST]}`,
		`not: [yaml`,
	}
	for i, raw := range cases {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("case %d should fail validation", i)
		}
	}
}

func TestLegalSuccessorsTable(t *testing.T) {
	e := loadRepoEngine(t)
	got := e.LegalSuccessors(domain.TypeBug, "FIXED")
	want := map[string]bool{"RESOLVED": true, "FIXING": true}
	if len(got) != 2 {
		t.Fatalf("FIXED successors %v", got)
	}
	for _, s := range got {
		if !want[s] {
			t.Fatalf("unexpected successor %s", s)
		}
	}
}
