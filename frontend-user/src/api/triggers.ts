import { http, unwrapList } from './http'
import type { Trigger, TriggerExecution } from './types'

export async function listTriggers(projectId: string): Promise<Trigger[]> {
  const res = await http.get<unknown>(`/v1/projects/${projectId}/triggers`)
  return unwrapList<Trigger>(res.data)
}

export async function listTriggerExecutions(projectId: string): Promise<TriggerExecution[]> {
  const res = await http.get<unknown>(`/v1/projects/${projectId}/trigger-executions`)
  return unwrapList<TriggerExecution>(res.data)
}
