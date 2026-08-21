import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { getBoard, rankIssue, transitionIssue } from '@/api/issues'
import type { BoardColumn, ColumnId, Issue } from '@/api/types'
import { COLUMNS, columnOf, statusForColumn } from '@/utils/workflow'
import { useToastStore } from './toast'

export interface BoardFilter {
  assignee: string
  type: string
  mine: boolean
}

function emptyColumns(): BoardColumn[] {
  return COLUMNS.map((c) => ({ id: c.id, label: c.label, hint: c.hint, issues: [] }))
}

function cloneColumns(cols: BoardColumn[]): BoardColumn[] {
  return cols.map((c) => ({ ...c, issues: c.issues.map((i) => ({ ...i })) }))
}

export const useBoardStore = defineStore('board', () => {
  const columns = ref<BoardColumn[]>(emptyColumns())
  const loading = ref(false)
  const filter = ref<BoardFilter>({ assignee: '', type: '', mine: false })
  const currentUserId = ref('')

  const filteredColumns = computed(() =>
    columns.value.map((col) => ({
      ...col,
      issues: col.issues.filter((issue) => {
        if (filter.value.type && issue.issue_type !== filter.value.type) return false
        if (filter.value.assignee && issue.assignee_id !== filter.value.assignee) return false
        if (filter.value.mine && issue.assignee_id !== currentUserId.value) return false
        return true
      }),
    })),
  )

  function normalize(raw: BoardColumn[] | undefined, fallbackIssues: Issue[] = []) {
    const base = emptyColumns()
    if (raw?.length) {
      for (const col of raw) {
        const slot = base.find((c) => c.id === col.id)
        const rawIssues = col.issues || (col as BoardColumn & { cards?: Issue[] }).cards || []
        if (slot) slot.issues = rawIssues.map((i) => ({ ...i, column: col.id }))
      }
    } else {
      for (const issue of fallbackIssues) {
        const colId = issue.column || columnOf(issue.status, issue.issue_type)
        base.find((c) => c.id === colId)?.issues.push(issue)
      }
    }
    columns.value = base
  }

  async function load(projectId: string) {
    loading.value = true
    try {
      const res = await getBoard(projectId)
      const data = res.data
      normalize(data.columns)
    } finally {
      loading.value = false
    }
  }

  function findIssue(id: string): { col: BoardColumn; index: number; issue: Issue } | null {
    for (const col of columns.value) {
      const index = col.issues.findIndex((i) => i.id === id)
      if (index >= 0) return { col, index, issue: col.issues[index] }
    }
    return null
  }

  async function moveIssue(issueId: string, toCol: ColumnId, toIndex: number) {
    const toast = useToastStore()
    const snapshot = cloneColumns(columns.value)
    const found = findIssue(issueId)
    if (!found) return
    const fromCol = found.col.id
    const issue = { ...found.issue }
    found.col.issues.splice(found.index, 1)
    const dest = columns.value.find((c) => c.id === toCol)
    if (!dest) {
      columns.value = snapshot
      return
    }
    dest.issues.splice(toIndex, 0, { ...issue, column: toCol })

    try {
      if (fromCol !== toCol) {
        const toStatus = statusForColumn(issue, toCol)
        const res = await transitionIssue(issue.id, toStatus, issue.version)
        const payload = res.data
        const updated = (payload && 'issue' in payload && payload.issue) ? payload.issue : (payload as Issue)
        const warnings = (payload && 'warnings' in payload && payload.warnings) || (res as typeof res & { meta?: { warnings?: string[] } }).meta?.warnings
        const loc = findIssue(issue.id)
        if (loc && updated?.id) {
          loc.col.issues[loc.index] = { ...loc.issue, ...updated, column: toCol }
        }
        if (warnings?.length) toast.warn(warnings.join('；'))
      }
      const loc = findIssue(issue.id)
      if (loc) {
        const rank = (toIndex + 1) * 1000
        await rankIssue(issue.id, rank, loc.issue.version)
        loc.issue.board_rank = rank
      }
    } catch (err) {
      columns.value = snapshot
      throw err
    }
  }

  function replaceIssue(next: Issue) {
    const loc = findIssue(next.id)
    if (!loc) return
    const destCol = next.column || columnOf(next.status, next.issue_type)
    if (loc.col.id !== destCol) {
      loc.col.issues.splice(loc.index, 1)
      columns.value.find((c) => c.id === destCol)?.issues.unshift(next)
      return
    }
    loc.col.issues[loc.index] = { ...loc.issue, ...next }
  }

  function prependIssue(issue: Issue) {
    const colId = issue.column || columnOf(issue.status, issue.issue_type)
    columns.value.find((c) => c.id === colId)?.issues.unshift(issue)
  }

  function columnMeta(id: ColumnId) {
    const col = filteredColumns.value.find((c) => c.id === id)
    const issues = col?.issues ?? []
    return {
      count: issues.length,
      points: issues.reduce((s, i) => s + (i.story_points || 0), 0),
    }
  }

  return {
    columns,
    filteredColumns,
    loading,
    filter,
    currentUserId,
    load,
    moveIssue,
    replaceIssue,
    prependIssue,
    columnMeta,
    normalize,
  }
})
