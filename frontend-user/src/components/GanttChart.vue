<script setup lang="ts">
import { computed } from 'vue'
import type { Dependency, GanttBar, Issue, Sprint } from '@/api/types'
import { addDays, beijingTodayISO, dateKey, daysBetween, formatBeijingDate } from '@/utils/time'
import { isOverdue, TYPE_BAR } from '@/utils/workflow'

const props = defineProps<{
  bars: GanttBar[]
  issues?: Issue[]
  dependencies: Dependency[]
  sprint?: Sprint | null
  zoom: 'day' | 'week' | 'month'
}>()

const ROW = 36
const LABEL_W = 168
const PAD_TOP = 40

const pxPerDay = computed(() => (props.zoom === 'day' ? 28 : props.zoom === 'week' ? 12 : 4))

interface RowBar {
  id: string
  key: string
  title: string
  start: string
  end: string
  overdue: boolean
  color: string
}

const rows = computed<RowBar[]>(() => {
  const fromBars = props.bars.length
    ? props.bars
    : (props.issues || []).map((i) => ({
        id: i.id,
        issue: i,
        issue_id: i.id,
        key: i.key,
        title: i.title,
        start_date: i.start_date || undefined,
        due_date: i.due_date || undefined,
        status: i.status,
        issue_type: i.issue_type,
      }))
  return fromBars
    .map((b) => {
      const issue = b.issue
      const start = b.start_date || issue?.start_date
      const end = b.due_date || issue?.due_date
      if (!start || !end) return null
      const id = b.issue_id || issue?.id || b.id || ''
      const type = b.issue_type || issue?.issue_type || 'TASK'
      return {
        id,
        key: b.key || issue?.key || id.slice(0, 8),
        title: b.title || issue?.title || '',
        start: start.slice(0, 10),
        end: end.slice(0, 10),
        overdue: isOverdue({
          due_date: end,
          status: b.status || issue?.status || '',
          issue_type: type,
        }),
        color: TYPE_BAR[type],
      }
    })
    .filter((x): x is RowBar => Boolean(x))
})

const range = computed(() => {
  const today = beijingTodayISO()
  const dates = rows.value.flatMap((r) => [r.start, r.end])
  if (props.sprint?.start_date) dates.push(props.sprint.start_date.slice(0, 10))
  if (props.sprint?.end_date) dates.push(props.sprint.end_date.slice(0, 10))
  dates.push(today)
  if (!dates.length) return { start: today, end: addDays(today, 14) }
  dates.sort()
  return { start: addDays(dates[0], -2), end: addDays(dates[dates.length - 1], 3) }
})

const dayCount = computed(() => Math.max(1, daysBetween(range.value.start, range.value.end)))
const width = computed(() => LABEL_W + dayCount.value * pxPerDay.value + 24)
const height = computed(() => PAD_TOP + Math.max(rows.value.length, 1) * ROW + 16)

function xOf(iso: string): number {
  return LABEL_W + daysBetween(range.value.start, iso) * pxPerDay.value
}

const todayX = computed(() => xOf(beijingTodayISO()))

const ticks = computed(() => {
  const out: { x: number; label: string }[] = []
  const step = props.zoom === 'day' ? 1 : props.zoom === 'week' ? 7 : 14
  for (let i = 0; i <= dayCount.value; i += step) {
    const d = addDays(range.value.start, i)
    out.push({ x: xOf(d), label: formatBeijingDate(d).slice(5) })
  }
  return out
})

const curves = computed(() => {
  return props.dependencies
    .filter((d) => d.dep_type === 'FS' || !d.dep_type)
    .map((d) => {
      const a = rows.value.findIndex((r) => r.id === d.predecessor_id)
      const b = rows.value.findIndex((r) => r.id === d.successor_id)
      if (a < 0 || b < 0) return null
      const pred = rows.value[a]
      const succ = rows.value[b]
      const x1 = xOf(pred.end) + 8
      const y1 = PAD_TOP + a * ROW + ROW / 2
      const x2 = xOf(succ.start)
      const y2 = PAD_TOP + b * ROW + ROW / 2
      const dx = Math.max(36, Math.abs(x2 - x1) * 0.4)
      return { id: d.id, path: `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}` }
    })
    .filter((x): x is { id: string; path: string } => Boolean(x))
})

const sprintBand = computed(() => {
  if (!props.sprint) return null
  const x = xOf(props.sprint.start_date.slice(0, 10))
  const w = Math.max(8, xOf(props.sprint.end_date.slice(0, 10)) - x)
  return { x, w }
})

function barWidth(row: RowBar) {
  return Math.max(pxPerDay.value, xOf(row.end) - xOf(row.start) + pxPerDay.value)
}
</script>

<template>
  <div class="scrollbar-thin overflow-auto rounded-card bg-ink-2 ring-1 ring-line">
    <svg v-if="rows.length" :width="width" :height="height" class="block min-w-full">
      <rect
        v-if="sprintBand"
        :x="sprintBand.x"
        :y="PAD_TOP"
        :width="sprintBand.w"
        :height="rows.length * ROW"
        fill="rgba(233,196,106,0.08)"
      />
      <g v-for="t in ticks" :key="t.label + t.x">
        <line :x1="t.x" y1="0" :x2="t.x" :y2="height" stroke="rgba(243,235,224,0.05)" />
        <text :x="t.x + 4" y="18" fill="#C9BFAF" font-size="11" font-family="IBM Plex Mono, monospace">
          {{ t.label }}
        </text>
      </g>
      <line :x1="todayX" y1="0" :x2="todayX" :y2="height" stroke="#D46A2C" stroke-width="2" />
      <text :x="todayX + 6" y="32" fill="#D46A2C" font-size="11">今日 {{ dateKey(new Date()) }}</text>

      <g v-for="(row, idx) in rows" :key="row.id">
        <text
          x="12"
          :y="PAD_TOP + idx * ROW + 22"
          fill="#F3EBE0"
          font-size="12"
          font-family="IBM Plex Mono, monospace"
        >
          {{ row.key }}
        </text>
        <rect
          :x="xOf(row.start)"
          :y="PAD_TOP + idx * ROW + 8"
          :width="barWidth(row)"
          height="20"
          rx="4"
          :fill="row.overdue ? '#C45C5C' : row.color"
          opacity="0.92"
        />
        <title>{{ row.title }}</title>
      </g>
      <path
        v-for="c in curves"
        :key="c.id"
        :d="c.path"
        fill="none"
        stroke="rgba(243,235,224,0.35)"
        stroke-width="1.4"
      />
    </svg>
    <div v-else class="grid h-64 place-items-center border border-dashed border-line text-sm text-paper-dim">
      当前没有带起止日期的事项
    </div>
  </div>
</template>
