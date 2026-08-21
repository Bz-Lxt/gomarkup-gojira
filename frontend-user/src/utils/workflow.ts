import type { ColumnId, Issue, IssueType, Role } from '@/api/types'

export const COLUMNS: { id: ColumnId; label: string; hint?: string; accent: string }[] = [
  { id: 'TODO', label: '待处理', accent: 'var(--paper-dim)' },
  { id: 'IN_PROGRESS', label: '开发中', accent: 'var(--teal)' },
  { id: 'TESTING', label: '已测试', hint: '测试验证中', accent: 'var(--gold)' },
  { id: 'DONE', label: '已完成', accent: 'var(--olive)' },
]

const TASK_STATUS_TO_COLUMN: Record<string, ColumnId> = {
  TODO: 'TODO',
  IN_PROGRESS: 'IN_PROGRESS',
  TESTING: 'TESTING',
  DONE: 'DONE',
}

const BUG_STATUS_TO_COLUMN: Record<string, ColumnId> = {
  NEW: 'TODO',
  CONFIRMED: 'TODO',
  FIXING: 'IN_PROGRESS',
  REOPENED: 'IN_PROGRESS',
  FIXED: 'TESTING',
  RESOLVED: 'DONE',
  CLOSED: 'DONE',
  REJECTED: 'DONE',
}

export function columnOf(status: string, type: IssueType): ColumnId {
  if (type === 'BUG') return BUG_STATUS_TO_COLUMN[status] ?? 'TODO'
  return TASK_STATUS_TO_COLUMN[status] ?? 'TODO'
}

export function statusForColumn(issue: Issue, col: ColumnId): string {
  if (columnOf(issue.status, issue.issue_type) === col) return issue.status
  if (issue.issue_type === 'BUG') {
    const map: Record<ColumnId, string> = {
      TODO: issue.status === 'NEW' ? 'NEW' : 'CONFIRMED',
      IN_PROGRESS: 'FIXING',
      TESTING: 'FIXED',
      DONE: 'RESOLVED',
    }
    return map[col]
  }
  return col
}

export interface TransitionDef {
  from: string
  to: string
  roles: Role[]
  label: string
}

export const TASK_TRANSITIONS: TransitionDef[] = [
  { from: 'TODO', to: 'IN_PROGRESS', roles: ['DEV', 'PM', 'ADMIN'], label: '开始开发' },
  { from: 'IN_PROGRESS', to: 'TESTING', roles: ['DEV', 'QA', 'ADMIN'], label: '提交测试' },
  { from: 'TESTING', to: 'DONE', roles: ['QA', 'ADMIN'], label: '验收完成' },
  { from: 'IN_PROGRESS', to: 'TODO', roles: ['PM', 'ADMIN'], label: '撤回待处理' },
  { from: 'TESTING', to: 'IN_PROGRESS', roles: ['QA', 'ADMIN'], label: '打回开发' },
  { from: 'DONE', to: 'TESTING', roles: ['PM', 'ADMIN'], label: '重新打开' },
]

export const BUG_TRANSITIONS: TransitionDef[] = [
  { from: 'NEW', to: 'CONFIRMED', roles: ['QA', 'PM', 'ADMIN'], label: '确认 Bug' },
  { from: 'NEW', to: 'REJECTED', roles: ['QA', 'PM'], label: '拒绝' },
  { from: 'CONFIRMED', to: 'FIXING', roles: ['DEV', 'ADMIN'], label: '开始修复' },
  { from: 'FIXING', to: 'FIXED', roles: ['DEV', 'ADMIN'], label: '标记已修复' },
  { from: 'FIXED', to: 'RESOLVED', roles: ['QA'], label: '验证通过（已解决）' },
  { from: 'FIXED', to: 'FIXING', roles: ['QA'], label: '验证不通过' },
  { from: 'RESOLVED', to: 'CLOSED', roles: ['PM', 'QA', 'ADMIN'], label: '关闭' },
  { from: 'CLOSED', to: 'REOPENED', roles: ['QA', 'PM', 'ADMIN'], label: '重新打开' },
  { from: 'REJECTED', to: 'REOPENED', roles: ['QA', 'PM', 'ADMIN'], label: '重新打开' },
  { from: 'REOPENED', to: 'FIXING', roles: ['DEV', 'ADMIN'], label: '继续修复' },
  { from: 'REOPENED', to: 'FIXED', roles: ['DEV', 'ADMIN'], label: '标记已修复' },
]

export function allowedTransitions(issue: Issue, role?: Role | null): TransitionDef[] {
  const table = issue.issue_type === 'BUG' ? BUG_TRANSITIONS : TASK_TRANSITIONS
  return table.filter((t) => t.from === issue.status && (!role || t.roles.includes(role)))
}

export const FIBONACCI = [0, 1, 2, 3, 5, 8, 13, 21] as const

export const PRIORITY_COLOR: Record<string, string> = {
  BLOCKER: 'var(--rose)',
  HIGHEST: 'var(--rose)',
  HIGH: 'var(--copper)',
  MEDIUM: 'var(--gold)',
  LOW: 'var(--teal)',
  LOWEST: 'var(--paper-dim)',
}

export const TYPE_BAR: Record<IssueType, string> = {
  STORY: 'var(--copper)',
  TASK: 'var(--teal)',
  BUG: 'var(--rose)',
}

export const TYPE_LABEL: Record<IssueType, string> = {
  STORY: '故事',
  TASK: '任务',
  BUG: '缺陷',
}

export const ROLE_COLOR: Record<Role, string> = {
  ADMIN: 'var(--copper)',
  PM: 'var(--gold)',
  DEV: 'var(--teal)',
  QA: 'var(--olive)',
  VIEWER: 'var(--paper-dim)',
}

export const ROLE_LABEL: Record<Role, string> = {
  ADMIN: '管理员',
  PM: '产品经理',
  DEV: '开发',
  QA: '测试',
  VIEWER: '访客',
}

export function issueKey(projectKey: string, seq: number): string {
  return `${projectKey}-${seq}`
}

export function initials(name?: string): string {
  if (!name) return '?'
  const trimmed = name.trim()
  if (!trimmed) return '?'
  return trimmed.slice(0, 1).toUpperCase()
}

export function isOverdue(issue: Pick<Issue, 'due_date' | 'status' | 'issue_type'>): boolean {
  if (!issue.due_date) return false
  const done =
    issue.status === 'DONE' ||
    issue.status === 'CLOSED' ||
    issue.status === 'RESOLVED' ||
    issue.status === 'REJECTED'
  if (done) return false
  const due = new Date(issue.due_date.includes('T') ? issue.due_date : `${issue.due_date}T23:59:59+08:00`)
  return due.getTime() < Date.now()
}
