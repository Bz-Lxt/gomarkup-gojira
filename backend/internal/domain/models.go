package domain

import "time"

const (
	RoleAdmin  = "ADMIN"
	RolePM     = "PM"
	RoleDev    = "DEV"
	RoleQA     = "QA"
	RoleViewer = "VIEWER"

	TypeStory = "STORY"
	TypeTask  = "TASK"
	TypeBug   = "BUG"

	SprintPlanned = "PLANNED"
	SprintActive  = "ACTIVE"
	SprintClosed  = "CLOSED"

	DepFS = "FS"
	DepSS = "SS"
	DepFF = "FF"
	DepSF = "SF"

	EventIssueStatusChanged = "issue.status_changed"
	EventIssueCreated       = "issue.created"
	EventIssueAssigned      = "issue.assigned"
	EventSprintStarted      = "sprint.started"
	EventSprintClosed       = "sprint.closed"
	EventIssueOverdue       = "issue.overdue"

	OutboxPending = "pending"
	OutboxDone    = "done"
	OutboxFailed  = "failed"
	OutboxDead    = "dead"

	ErrorTransient = "transient"
	ErrorPermanent = "permanent"

	ActionSendEmail   = "SEND_EMAIL"
	ActionWebhook     = "WEBHOOK"
	ActionAutoAssign  = "AUTO_ASSIGN"
	ActionAddComment  = "ADD_COMMENT"
	ActionInAppNotify = "IN_APP_NOTIFY"
)

var StoryPointSet = map[int]struct{}{0: {}, 1: {}, 2: {}, 3: {}, 5: {}, 8: {}, 13: {}, 21: {}}

var Priorities = map[string]struct{}{"LOWEST": {}, "LOW": {}, "MEDIUM": {}, "HIGH": {}, "HIGHEST": {}}

var Severities = map[string]struct{}{"BLOCKER": {}, "CRITICAL": {}, "MAJOR": {}, "MINOR": {}, "TRIVIAL": {}}

var DoneStatuses = map[string]struct{}{
	"DONE": {}, "RESOLVED": {}, "CLOSED": {}, "REJECTED": {},
}

func IsDoneStatus(status string) bool {
	_, ok := DoneStatuses[status]
	return ok
}

func ValidStoryPoints(n int) bool {
	_, ok := StoryPointSet[n]
	return ok
}

func ValidRole(role string) bool {
	switch role {
	case RoleAdmin, RolePM, RoleDev, RoleQA, RoleViewer:
		return true
	default:
		return false
	}
}

func ValidIssueType(t string) bool {
	return t == TypeStory || t == TypeTask || t == TypeBug
}

func ValidDepType(t string) bool {
	return t == DepFS || t == DepSS || t == DepFF || t == DepSF
}

type User struct {
	ID           int64     `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	DisplayName  string    `db:"display_name" json:"display_name"`
	AvatarURL    *string   `db:"avatar_url" json:"avatar_url"`
	Role         string    `db:"role" json:"role"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type Project struct {
	ID                     int64     `db:"id" json:"id"`
	Key                    string    `db:"key" json:"key"`
	Name                   string    `db:"name" json:"name"`
	Description            string    `db:"description" json:"description"`
	OwnerID                int64     `db:"owner_id" json:"owner_id"`
	EnforceDependencyBlock bool      `db:"enforce_dependency_block" json:"enforce_dependency_block"`
	WorkflowConfig         string    `db:"workflow_config" json:"workflow_config"`
	CreatedAt              time.Time `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time `db:"updated_at" json:"updated_at"`
}

type ProjectMember struct {
	ProjectID int64     `db:"project_id" json:"project_id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	Role      string    `db:"role" json:"role"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	Username  string    `db:"username" json:"username,omitempty"`
	Email     string    `db:"email" json:"email,omitempty"`
	Display   string    `db:"display_name" json:"display_name,omitempty"`
}

type Sprint struct {
	ID              int64     `db:"id" json:"id"`
	ProjectID       int64     `db:"project_id" json:"project_id"`
	Name            string    `db:"name" json:"name"`
	Goal            string    `db:"goal" json:"goal"`
	StartDate       time.Time `db:"start_date" json:"start_date"`
	EndDate         time.Time `db:"end_date" json:"end_date"`
	Status          string    `db:"status" json:"status"`
	CommittedPoints int       `db:"committed_points" json:"committed_points"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

type Issue struct {
	ID             int64      `db:"id" json:"id"`
	ProjectID      int64      `db:"project_id" json:"project_id"`
	SeqNo          int        `db:"seq_no" json:"seq_no"`
	IssueType      string     `db:"issue_type" json:"issue_type"`
	Title          string     `db:"title" json:"title"`
	Description    string     `db:"description" json:"description"`
	Status         string     `db:"status" json:"status"`
	Priority       string     `db:"priority" json:"priority"`
	Severity       *string    `db:"severity" json:"severity"`
	ReproduceSteps *string    `db:"reproduce_steps" json:"reproduce_steps"`
	AffectVersion  *string    `db:"affect_version" json:"affect_version"`
	FixVersion     *string    `db:"fix_version" json:"fix_version"`
	AssigneeID     *int64     `db:"assignee_id" json:"assignee_id"`
	ReporterID     int64      `db:"reporter_id" json:"reporter_id"`
	SprintID       *int64     `db:"sprint_id" json:"sprint_id"`
	StoryPoints    *int       `db:"story_points" json:"story_points"`
	EstimateHours  *float64   `db:"estimate_hours" json:"estimate_hours"`
	StartDate      *time.Time `db:"start_date" json:"start_date"`
	DueDate        *time.Time `db:"due_date" json:"due_date"`
	BoardRank      float64    `db:"board_rank" json:"board_rank"`
	Version        int        `db:"version" json:"version"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
	ResolvedAt     *time.Time `db:"resolved_at" json:"resolved_at"`
	Key            string     `db:"issue_key" json:"key"`
	ProjectKey     string     `db:"project_key" json:"project_key,omitempty"`
	Labels         []string   `json:"labels,omitempty"`
}

func FormatKey(projectKey string, seq int) string {
	return projectKey + "-" + itoa(seq)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

type StatusHistory struct {
	ID          int64     `db:"id" json:"id"`
	IssueID     int64     `db:"issue_id" json:"issue_id"`
	FromStatus  string    `db:"from_status" json:"from_status"`
	ToStatus    string    `db:"to_status" json:"to_status"`
	ActorID     int64     `db:"actor_id" json:"actor_id"`
	ChangedAt   time.Time `db:"changed_at" json:"changed_at"`
	DurationSec int       `db:"duration_sec" json:"duration_sec"`
	ActorName   string    `db:"actor_name" json:"actor_name,omitempty"`
}

type Comment struct {
	ID        int64     `db:"id" json:"id"`
	IssueID   int64     `db:"issue_id" json:"issue_id"`
	AuthorID  int64     `db:"author_id" json:"author_id"`
	Body      string    `db:"body" json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	Author    string    `db:"author_name" json:"author_name,omitempty"`
}

type Dependency struct {
	ID            int64  `db:"id" json:"id"`
	PredecessorID int64  `db:"predecessor_id" json:"predecessor_id"`
	SuccessorID   int64  `db:"successor_id" json:"successor_id"`
	DepType       string `db:"dep_type" json:"dep_type"`
	PredKey       string `db:"pred_key" json:"predecessor_key,omitempty"`
	SuccKey       string `db:"succ_key" json:"successor_key,omitempty"`
}

type Trigger struct {
	ID        int64     `db:"id" json:"id"`
	ProjectID int64     `db:"project_id" json:"project_id"`
	Name      string    `db:"name" json:"name"`
	EventType string    `db:"event_type" json:"event_type"`
	Condition []byte    `db:"condition" json:"condition"`
	Actions   []byte    `db:"actions" json:"actions"`
	IsEnabled bool      `db:"is_enabled" json:"is_enabled"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type OutboxEvent struct {
	ID            int64      `db:"id" json:"id"`
	EventID       string     `db:"event_id" json:"event_id"`
	EventType     string     `db:"event_type" json:"event_type"`
	Payload       []byte     `db:"payload" json:"payload"`
	Status        string     `db:"status" json:"status"`
	RetryCount    int        `db:"retry_count" json:"retry_count"`
	ErrorClass    *string    `db:"error_class" json:"error_class"`
	ErrorMsg      *string    `db:"error_msg" json:"error_msg"`
	NextAttemptAt *time.Time `db:"next_attempt_at" json:"next_attempt_at"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

type TriggerExecution struct {
	ID         int64     `db:"id" json:"id"`
	TriggerID  int64     `db:"trigger_id" json:"trigger_id"`
	EventID    string    `db:"event_id" json:"event_id"`
	ActionType string    `db:"action_type" json:"action_type"`
	Status     string    `db:"status" json:"status"`
	ErrorClass *string   `db:"error_class" json:"error_class"`
	ErrorMsg   *string   `db:"error_msg" json:"error_msg"`
	RetryCount int       `db:"retry_count" json:"retry_count"`
	DurationMS int       `db:"duration_ms" json:"duration_ms"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

type Notification struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	Type      string    `db:"type" json:"type"`
	Title     string    `db:"title" json:"title"`
	Body      string    `db:"body" json:"body"`
	IsRead    bool      `db:"is_read" json:"is_read"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type BoardColumn struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Hint   string  `json:"hint,omitempty"`
	Count  int     `json:"count"`
	Points int     `json:"points"`
	Cards  []Issue `json:"issues"`
}

type BoardView struct {
	Columns []BoardColumn `json:"columns"`
}

type GanttBar struct {
	IssueID    int64      `json:"issue_id"`
	Key        string     `json:"key"`
	Title      string     `json:"title"`
	IssueType  string     `json:"issue_type"`
	Status     string     `json:"status"`
	StartDate  *time.Time `json:"start_date"`
	DueDate    *time.Time `json:"due_date"`
	Overdue    bool       `json:"overdue"`
	AssigneeID *int64     `json:"assignee_id"`
}

type GanttView struct {
	Bars         []GanttBar   `json:"bars"`
	Dependencies []Dependency `json:"dependencies"`
	Today        string       `json:"today"`
}

type StatusChangedPayload struct {
	EventID    string `json:"event_id"`
	IssueID    int64  `json:"issue_id"`
	ProjectID  int64  `json:"project_id"`
	IssueKey   string `json:"issue_key"`
	Title      string `json:"title"`
	IssueType  string `json:"issue_type"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	ToColumn   string `json:"to_column"`
	ActorID    int64  `json:"actor_id"`
}
