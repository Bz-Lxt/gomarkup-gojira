import { http, unwrapList } from './http'
import type { BoardData, Comment, Dependency, Issue, IssueDraft, StatusHistory, TransitionResult } from './types'

export async function listIssues(projectId: string, params?: Record<string, string | number | undefined>) {
  const res = await http.get<unknown>(`/v1/projects/${projectId}/issues`, { params })
  return unwrapList<Issue>(res.data)
}

export function createIssue(projectId: string, payload: IssueDraft) {
  return http.post<Issue>(`/v1/projects/${projectId}/issues`, payload)
}

export function getIssue(id: string) {
  return http.get<Issue>(`/v1/issues/${id}`)
}

export function updateIssue(id: string, payload: Partial<IssueDraft> & { version?: number; sprint_id?: string | null }) {
  return http.patch<Issue>(`/v1/issues/${id}`, payload)
}

export function transitionIssue(id: string, to: string, version: number) {
  return http.post<TransitionResult | Issue>(`/v1/issues/${id}/transition`, { to, version })
}

export function rankIssue(id: string, board_rank: number, version?: number) {
  return http.patch<Issue>(`/v1/issues/${id}/rank`, { board_rank, version })
}

export function getBoard(projectId: string) {
  return http.get<BoardData>(`/v1/projects/${projectId}/board`)
}

export async function listComments(issueId: string): Promise<Comment[]> {
  const res = await http.get<unknown>(`/v1/issues/${issueId}/comments`)
  return unwrapList<Comment>(res.data)
}

export function addComment(issueId: string, body: string) {
  return http.post<Comment>(`/v1/issues/${issueId}/comments`, { body })
}

export async function listHistory(issueId: string): Promise<StatusHistory[]> {
  const res = await http.get<unknown>(`/v1/issues/${issueId}/history`)
  return unwrapList<StatusHistory>(res.data)
}

export async function listDependencies(issueId: string): Promise<Dependency[]> {
  const res = await http.get<unknown>(`/v1/issues/${issueId}/dependencies`)
  return unwrapList<Dependency>(res.data)
}

export function addDependency(issueId: string, payload: { predecessor_id?: string; successor_id?: string; dep_type: string }) {
  return http.post<Dependency>(`/v1/issues/${issueId}/dependencies`, payload)
}

export function removeDependency(issueId: string, depId: string) {
  return http.delete(`/v1/issues/${issueId}/dependencies/${depId}`)
}
