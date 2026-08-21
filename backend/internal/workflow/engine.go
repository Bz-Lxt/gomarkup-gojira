package workflow

import (
	"fmt"
	"os"
	"strings"

	"gojira/internal/domain"

	"gopkg.in/yaml.v3"
)

type GuardFunc func(issue *domain.Issue) error

var guardRegistry = map[string]GuardFunc{}

func init() {
	RegisterGuard("has_assignee", func(issue *domain.Issue) error {
		if issue == nil || issue.AssigneeID == nil || *issue.AssigneeID == 0 {
			return domain.GuardFailed("has_assignee", "必须指定经办人才能进入该状态")
		}
		return nil
	})
	RegisterGuard("dependencies_satisfied", func(issue *domain.Issue) error {
		// Soft/hard enforcement is applied by the issue service using live DB
		// edges. The registered name exists so YAML validation succeeds.
		return nil
	})
}

func RegisterGuard(name string, fn GuardFunc) {
	guardRegistry[name] = fn
}

func GuardNames() []string {
	out := make([]string, 0, len(guardRegistry))
	for k := range guardRegistry {
		out = append(out, k)
	}
	return out
}

type File struct {
	Version   int                 `yaml:"version"`
	Roles     []string            `yaml:"roles"`
	Columns   []Column            `yaml:"columns"`
	Workflows map[string]Workflow `yaml:"workflows"`
}

type Column struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
	Hint  string `yaml:"hint"`
}

type Workflow struct {
	AppliesTo   []string         `yaml:"applies_to"`
	Initial     string           `yaml:"initial"`
	States      map[string]State `yaml:"states"`
	Transitions []Transition     `yaml:"transitions"`
}

type State struct {
	Column   string `yaml:"column"`
	Terminal bool   `yaml:"terminal"`
}

type Transition struct {
	From         string   `yaml:"from"`
	To           string   `yaml:"to"`
	AllowedRoles []string `yaml:"allowed_roles"`
	Guards       []string `yaml:"guards"`
}

type Engine struct {
	File      File
	byType    map[string]string
	roleSet   map[string]struct{}
	columnSet map[string]Column
}

func Load(path string) (*Engine, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workflow %s: %w", path, err)
	}
	return Parse(raw)
}

func Parse(raw []byte) (*Engine, error) {
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse workflow yaml: %w", err)
	}
	e := &Engine{File: f}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	e.index()
	return e, nil
}

func (e *Engine) index() {
	e.byType = map[string]string{}
	e.roleSet = map[string]struct{}{}
	e.columnSet = map[string]Column{}
	for _, r := range e.File.Roles {
		e.roleSet[r] = struct{}{}
	}
	for _, c := range e.File.Columns {
		e.columnSet[c.ID] = c
	}
	for name, wf := range e.File.Workflows {
		for _, t := range wf.AppliesTo {
			e.byType[t] = name
		}
	}
}

func (e *Engine) Validate() error {
	if e.File.Version < 1 {
		return fmt.Errorf("workflow version must be >= 1")
	}
	if len(e.File.Roles) == 0 {
		return fmt.Errorf("workflow roles must not be empty")
	}
	roles := map[string]struct{}{}
	for _, r := range e.File.Roles {
		roles[r] = struct{}{}
	}
	cols := map[string]struct{}{}
	for _, c := range e.File.Columns {
		if c.ID == "" {
			return fmt.Errorf("column id is required")
		}
		cols[c.ID] = struct{}{}
	}
	if len(e.File.Workflows) == 0 {
		return fmt.Errorf("at least one workflow is required")
	}
	for name, wf := range e.File.Workflows {
		if err := validateWorkflow(name, wf, roles, cols); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkflow(name string, wf Workflow, roles, cols map[string]struct{}) error {
	if wf.Initial == "" {
		return fmt.Errorf("%s: initial state is required", name)
	}
	if _, ok := wf.States[wf.Initial]; !ok {
		return fmt.Errorf("%s: initial state %s is not declared", name, wf.Initial)
	}
	hasTerminal := false
	for st, meta := range wf.States {
		if meta.Column == "" {
			return fmt.Errorf("%s: state %s missing column", name, st)
		}
		if _, ok := cols[meta.Column]; !ok {
			return fmt.Errorf("%s: state %s maps to unknown column %s", name, st, meta.Column)
		}
		if meta.Terminal {
			hasTerminal = true
		}
	}
	if !hasTerminal {
		return fmt.Errorf("%s: at least one terminal state is required", name)
	}
	adj := map[string][]string{}
	seen := map[string]struct{}{}
	for i, t := range wf.Transitions {
		if _, ok := wf.States[t.From]; !ok {
			return fmt.Errorf("%s: transition[%d] from unknown state %s", name, i, t.From)
		}
		if _, ok := wf.States[t.To]; !ok {
			return fmt.Errorf("%s: transition[%d] to unknown state %s", name, i, t.To)
		}
		if len(t.AllowedRoles) == 0 {
			return fmt.Errorf("%s: transition %s→%s has no allowed_roles", name, t.From, t.To)
		}
		for _, r := range t.AllowedRoles {
			if _, ok := roles[r]; !ok {
				return fmt.Errorf("%s: transition %s→%s references unknown role %s", name, t.From, t.To, r)
			}
		}
		for _, g := range t.Guards {
			if _, ok := guardRegistry[g]; !ok {
				return fmt.Errorf("%s: transition %s→%s references unregistered guard %s", name, t.From, t.To, g)
			}
		}
		adj[t.From] = append(adj[t.From], t.To)
		seen[t.From] = struct{}{}
		seen[t.To] = struct{}{}
	}
	reachable := map[string]struct{}{}
	var walk func(string)
	walk = func(s string) {
		if _, ok := reachable[s]; ok {
			return
		}
		reachable[s] = struct{}{}
		for _, n := range adj[s] {
			walk(n)
		}
	}
	walk(wf.Initial)
	var orphans []string
	for st := range wf.States {
		if _, ok := reachable[st]; !ok {
			orphans = append(orphans, st)
		}
	}
	if len(orphans) > 0 {
		return fmt.Errorf("%s: unreachable states: %s", name, strings.Join(orphans, ","))
	}
	return nil
}

func (e *Engine) WorkflowFor(issueType string) (string, Workflow, error) {
	name, ok := e.byType[issueType]
	if !ok {
		return "", Workflow{}, domain.Unprocessable(domain.CodeUnprocessable, "未知事项类型", issueType)
	}
	return name, e.File.Workflows[name], nil
}

func (e *Engine) InitialStatus(issueType string) (string, error) {
	_, wf, err := e.WorkflowFor(issueType)
	if err != nil {
		return "", err
	}
	return wf.Initial, nil
}

func (e *Engine) ColumnOf(issueType, status string) (string, bool) {
	_, wf, err := e.WorkflowFor(issueType)
	if err != nil {
		return "", false
	}
	st, ok := wf.States[status]
	if !ok {
		return "", false
	}
	return st.Column, true
}

func (e *Engine) ColumnMeta(id string) (Column, bool) {
	c, ok := e.columnSet[id]
	return c, ok
}

func (e *Engine) Columns() []Column {
	return e.File.Columns
}

func (e *Engine) LegalSuccessors(issueType, from string) []string {
	_, wf, err := e.WorkflowFor(issueType)
	if err != nil {
		return nil
	}
	var out []string
	for _, t := range wf.Transitions {
		if t.From == from {
			out = append(out, t.To)
		}
	}
	return out
}

func (e *Engine) FindTransition(issueType, from, to string) (Transition, bool) {
	_, wf, err := e.WorkflowFor(issueType)
	if err != nil {
		return Transition{}, false
	}
	for _, t := range wf.Transitions {
		if t.From == from && t.To == to {
			return t, true
		}
	}
	return Transition{}, false
}

// CheckTransition validates edge, role and guards. It does not touch the database.
func (e *Engine) CheckTransition(issue *domain.Issue, actorProjectRole, to string) (Transition, error) {
	if issue == nil {
		return Transition{}, domain.NotFound("事项不存在")
	}
	if to == "" {
		return Transition{}, domain.BadRequest(domain.CodeInvalidInput, "缺少目标状态", nil)
	}
	tr, ok := e.FindTransition(issue.IssueType, issue.Status, to)
	if !ok {
		return Transition{}, domain.InvalidTransition(e.LegalSuccessors(issue.IssueType, issue.Status))
	}
	if !roleAllowed(tr.AllowedRoles, actorProjectRole) {
		return Transition{}, domain.Forbidden(
			fmt.Sprintf("角色 %s 不能将状态从 %s 转为 %s", actorProjectRole, issue.Status, to),
			map[string]any{"allowed_roles": tr.AllowedRoles, "from": issue.Status, "to": to},
		)
	}
	for _, g := range tr.Guards {
		fn, ok := guardRegistry[g]
		if !ok {
			return Transition{}, domain.GuardFailed(g, "守卫未注册")
		}
		if err := fn(issue); err != nil {
			return Transition{}, err
		}
	}
	return tr, nil
}

func roleAllowed(allowed []string, role string) bool {
	for _, a := range allowed {
		if a == role {
			return true
		}
	}
	return false
}

func (e *Engine) IsColumn(issueType, status, column string) bool {
	col, ok := e.ColumnOf(issueType, status)
	return ok && col == column
}
