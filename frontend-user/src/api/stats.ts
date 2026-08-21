import { http } from './http'
import type { BugStats, BurndownPoint, Metric, Progress, VelocityPoint } from './types'
import { unwrapList } from './http'

export async function getBurndown(sprintId: string, metric: Metric): Promise<BurndownPoint[]> {
  const res = await http.get<unknown>(`/v1/sprints/${sprintId}/burndown`, { params: { metric } })
  const raw = res.data as {
    ideal?: { date: string; value: number }[]
    actual?: { date: string; value: number }[]
    scope_changes?: { date: string; delta: number }[]
  } | BurndownPoint[]
  if (Array.isArray(raw)) {
    return raw.map((p) => ({
      date: p.date || (p as unknown as { day?: string }).day || '',
      ideal: p.ideal,
      actual: p.actual ?? (p as unknown as { remaining?: number }).remaining ?? 0,
      scope_change: p.scope_change,
    }))
  }
  const scope = new Map((raw.scope_changes || []).map((s) => [s.date, s.delta]))
  return (raw.actual || []).map((a, i) => ({
    date: a.date,
    actual: a.value,
    ideal: raw.ideal?.[i]?.value ?? 0,
    scope_change: scope.get(a.date),
  }))
}

export async function getVelocity(projectId: string): Promise<VelocityPoint[]> {
  const res = await http.get<unknown>(`/v1/projects/${projectId}/velocity`)
  return unwrapList<VelocityPoint>(res.data).map((p) => ({
    ...p,
    week: p.week || (p as unknown as { iso_week?: string }).iso_week || '',
    display_name: p.display_name || (p as unknown as { assignee_name?: string }).assignee_name,
    count: p.count ?? (p as unknown as { issue_count?: number }).issue_count ?? 0,
  }))
}

export function getProgress(sprintId: string) {
  return http.get<Progress>(`/v1/sprints/${sprintId}/progress`)
}

export function getBugStats(projectId: string) {
  return http.get<BugStats>(`/v1/projects/${projectId}/bug-stats`)
}
