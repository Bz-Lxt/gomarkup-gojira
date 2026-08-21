export type Role = 'ADMIN' | 'PM' | 'DEV' | 'QA' | 'VIEWER'
export type IssueType = 'STORY' | 'TASK' | 'BUG'
export type Priority = 'BLOCKER' | 'HIGHEST' | 'HIGH' | 'MEDIUM' | 'LOW' | 'LOWEST'
export type Severity = 'BLOCKER' | 'CRITICAL' | 'MAJOR' | 'MINOR' | 'TRIVIAL'
export type ColumnId = 'TODO' | 'IN_PROGRESS' | 'TESTING' | 'DONE'
export type SprintStatus = 'PLANNED' | 'ACTIVE' | 'CLOSED'
export type DepType = 'FS' | 'SS' | 'FF' | 'SF'
export type Metric = 'points' | 'hours' | 'count'

export interface Envelope<T> {
  data: T
  meta?: Record<string, unknown> & { warnings?: string[] }
}

export interface ApiErrorBody {
  code?: string
  message?: string
  details?: unknown
  request_id?: string
}

export class ApiError extends Error {
  code: string
  details: unknown
  requestId: string
  status: number

  constructor(status: number, body: ApiErrorBody) {
    super(body.message || '请求失败')
    this.name = 'ApiError'
    this.status = status
    this.code = body.code || 'UNKNOWN'
    this.details = body.details
    this.requestId = body.request_id || ''
  }
}

export interface User {
  id: string
  username: string
  email: string
  display_name: string
  avatar_url?: string
  role: Role
}

export interface AuthPayload {
  token?: string
  access_token?: string
  refresh_token: string
  user: User
}

export interface Project {
  id: string
  key: string
  name: string
  description: string
  owner_id: string
  enforce_dependency_block?: boolean
  member_count?: number
  issue_count?: number
  my_role?: Role
  created_at?: string
}

export interface Member {
  user_id: string
  role: Role
  user?: User
  display_name?: string
  username?: string
}

export interface Sprint {
  id: string
  project_id: string
  name: string
  goal: string
  start_date: string
  end_date: string
  status: SprintStatus
  committed_points?: number
}

export interface Issue {
  id: string
  project_id: string
  key?: string
  seq_no: number
  issue_type: IssueType
  title: string
  description: string
  status: string
  column?: ColumnId
  priority: Priority
  severity?: Severity
  assignee_id?: string | null
  assignee?: User | null
  reporter_id?: string
  reporter?: User | null
  sprint_id?: string | null
  story_points: number
  estimate_hours: number
  start_date?: string | null
  due_date?: string | null
  board_rank?: number
  version: number
  labels?: string[]
  reproduce_steps?: string
  affected_version?: string
  fix_version?: string
  created_at?: string
  updated_at?: string
}

export interface BoardColumn {
  id: ColumnId
  label?: string
  hint?: string
  issues: Issue[]
  count?: number
  points?: number
}

export interface BoardData {
  project?: Project
  sprint?: Sprint | null
  columns: BoardColumn[]
}

export interface Comment {
  id: string
  issue_id: string
  author?: User
  author_id?: string
  body: string
  created_at: string
}

export interface StatusHistory {
  id: string
  issue_id: string
  from_status: string
  to_status: string
  actor?: User
  actor_id?: string
  changed_at: string
  duration_sec?: number
}

export interface Dependency {
  id: string
  predecessor_id: string
  successor_id: string
  dep_type: DepType
}

export interface GanttBar {
  id?: string
  issue?: Issue
  issue_id?: string
  key?: string
  title?: string
  start_date?: string
  due_date?: string
  status?: string
  issue_type?: IssueType
}

export interface GanttData {
  bars?: GanttBar[]
  issues?: Issue[]
  dependencies: Dependency[]
  sprint?: Sprint | null
}

export interface BurndownPoint {
  date: string
  ideal: number
  actual: number
  scope_change?: number
}

export interface VelocityPoint {
  week: string
  user_id?: string
  display_name?: string
  points: number
  count: number
}

export interface Progress {
  completion_rate: number
  remaining_days: number
  velocity: number
  predicted_done_date?: string
  predicted_end?: string
  committed_points?: number
  completed_points?: number
}

export interface BugStats {
  by_severity: Record<string, number>
  by_status: Record<string, number>
  mttr_hours: number
}

export interface Trigger {
  id: string
  name: string
  event_type: string
  is_enabled: boolean
  condition?: unknown
  actions?: unknown
}

export interface TriggerExecution {
  id: string
  trigger_id: string
  trigger_name?: string
  event_id: string
  action_type: string
  status: string
  error_class?: string
  error_msg?: string
  retry_count: number
  duration_ms: number
  created_at: string
}

export interface IssueDraft {
  title: string
  description: string
  issue_type: IssueType
  priority: Priority
  severity?: Severity
  assignee_id?: string | null
  sprint_id?: string | null
  story_points: number
  estimate_hours: number
  start_date?: string | null
  due_date?: string | null
  labels?: string[]
  reproduce_steps?: string
  affected_version?: string
  fix_version?: string
}

export interface TransitionResult {
  issue?: Issue
  warnings?: string[]
}

export const SEED_ACCOUNTS = [
  { username: 'admin', password: 'Admin@123', role: 'ADMIN' as Role, hint: '管理员' },
  { username: 'pm', password: 'Pm@123456', role: 'PM' as Role, hint: '产品经理' },
  { username: 'dev', password: 'Dev@123456', role: 'DEV' as Role, hint: '开发' },
  { username: 'qa', password: 'Qa@123456', role: 'QA' as Role, hint: '测试' },
]
