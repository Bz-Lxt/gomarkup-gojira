import { http, unwrapList } from './http'
import type { Issue, Sprint } from './types'

export async function listSprints(projectId: string): Promise<Sprint[]> {
  const res = await http.get<unknown>(`/v1/projects/${projectId}/sprints`)
  return unwrapList<Sprint>(res.data)
}

export function createSprint(projectId: string, payload: Partial<Sprint>) {
  return http.post<Sprint>(`/v1/projects/${projectId}/sprints`, payload)
}

export function startSprint(id: string) {
  return http.post<Sprint>(`/v1/sprints/${id}/start`)
}

export function closeSprint(id: string, payload?: { move_to?: string }) {
  return http.post<Sprint>(`/v1/sprints/${id}/close`, payload ?? {})
}

export function addIssueToSprint(sprintId: string, issueId: string) {
  return http.post<Issue>(`/v1/sprints/${sprintId}/issues`, { issue_id: issueId })
}

export function removeIssueFromSprint(sprintId: string, issueId: string) {
  return http.delete(`/v1/sprints/${sprintId}/issues/${issueId}`)
}
