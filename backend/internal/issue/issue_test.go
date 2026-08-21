package issue

import (
	"testing"

	"gojira/internal/domain"
	"gojira/internal/workflow"
)

func TestFormatKey(t *testing.T) {
	if domain.FormatKey("GJ", 12) != "GJ-12" {
		t.Fatal(domain.FormatKey("GJ", 12))
	}
	if domain.FormatKey("ABC", 1) != "ABC-1" {
		t.Fatal()
	}
}

func TestValidStoryPoints(t *testing.T) {
	ok := []int{0, 1, 2, 3, 5, 8, 13, 21}
	for _, n := range ok {
		if !ValidStoryPointsPtr(&n) {
			t.Fatalf("%d should be valid", n)
		}
	}
	bad := []int{4, 6, 7, 9, 10, 20}
	for _, n := range bad {
		if ValidStoryPointsPtr(&n) {
			t.Fatalf("%d should be invalid", n)
		}
	}
	if !ValidStoryPointsPtr(nil) {
		t.Fatal("nil is allowed")
	}
}

func TestMidRank(t *testing.T) {
	a, b := 1000.0, 2000.0
	if MidRank(&a, &b) != 1500 {
		t.Fatal(MidRank(&a, &b))
	}
	if MidRank(&a, nil) != 2000 {
		t.Fatal(MidRank(&a, nil))
	}
	if MidRank(nil, &b) != 1000 {
		t.Fatal(MidRank(nil, &b))
	}
	if MidRank(nil, nil) != 1000 {
		t.Fatal(MidRank(nil, nil))
	}
}

func TestTransitionUsesEngine(t *testing.T) {
	e, err := workflow.Parse([]byte(`
version: 1
roles: [ADMIN, PM, DEV, QA, VIEWER]
columns:
  - {id: TODO, label: t}
  - {id: IN_PROGRESS, label: i}
  - {id: TESTING, label: s}
  - {id: DONE, label: d}
workflows:
  task_workflow:
    applies_to: [TASK]
    initial: TODO
    states:
      TODO: {column: TODO}
      IN_PROGRESS: {column: IN_PROGRESS}
      TESTING: {column: TESTING}
      DONE: {column: DONE, terminal: true}
    transitions:
      - {from: TODO, to: IN_PROGRESS, allowed_roles: [DEV], guards: [has_assignee]}
      - {from: IN_PROGRESS, to: TESTING, allowed_roles: [DEV]}
      - {from: TESTING, to: DONE, allowed_roles: [QA]}
  bug_workflow:
    applies_to: [BUG]
    initial: NEW
    states:
      NEW: {column: TODO}
      FIXED: {column: TESTING}
      RESOLVED: {column: DONE, terminal: true}
    transitions:
      - {from: NEW, to: FIXED, allowed_roles: [DEV]}
      - {from: FIXED, to: RESOLVED, allowed_roles: [QA]}
`))
	if err != nil {
		t.Fatal(err)
	}
	aid := int64(3)
	iss := &domain.Issue{IssueType: domain.TypeTask, Status: "TODO", AssigneeID: &aid}
	if _, err := e.CheckTransition(iss, domain.RoleDev, "IN_PROGRESS"); err != nil {
		t.Fatal(err)
	}
	bug := &domain.Issue{IssueType: domain.TypeBug, Status: "FIXED"}
	if _, err := e.CheckTransition(bug, domain.RoleDev, "RESOLVED"); err == nil {
		t.Fatal("DEV must not resolve")
	}
}

func TestNewEventIDUnique(t *testing.T) {
	a := NewEventID("x")
	b := NewEventID("x")
	if a == "" || a == b {
		t.Fatalf("%s %s", a, b)
	}
}

func TestIsDoneStatus(t *testing.T) {
	if !domain.IsDoneStatus("RESOLVED") || domain.IsDoneStatus("FIXED") {
		t.Fatal("FIXED is not a done-column terminal for remaining? RESOLVED is done, FIXED is testing")
	}
}
