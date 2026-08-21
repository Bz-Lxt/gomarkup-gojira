import { http } from './http'
import type { GanttData } from './types'

export function getGantt(projectId: string) {
  return http.get<GanttData>(`/v1/projects/${projectId}/gantt`)
}
