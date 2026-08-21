import { http, unwrapList } from './http'
import type { Member, Project } from './types'

export async function listProjects(): Promise<Project[]> {
  const res = await http.get<unknown>('/v1/projects')
  return unwrapList<Project>(res.data)
}

export function getProject(id: string) {
  return http.get<Project>(`/v1/projects/${id}`)
}

export function createProject(payload: { key: string; name: string; description: string }) {
  return http.post<Project>('/v1/projects', payload)
}

export function updateProject(id: string, payload: Partial<Project>) {
  return http.patch<Project>(`/v1/projects/${id}`, payload)
}

export async function listMembers(projectId: string): Promise<Member[]> {
  const res = await http.get<unknown>(`/v1/projects/${projectId}/members`)
  return unwrapList<Member>(res.data)
}

export function addMember(projectId: string, payload: { user_id: string; role: string }) {
  return http.post<Member>(`/v1/projects/${projectId}/members`, payload)
}

export function removeMember(projectId: string, userId: string) {
  return http.delete(`/v1/projects/${projectId}/members/${userId}`)
}
