<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import VChart from 'vue-echarts'
import BurndownChart from '@/components/BurndownChart.vue'
import VelocityChart from '@/components/VelocityChart.vue'
import { getBugStats, getBurndown, getProgress, getVelocity } from '@/api/stats'
import type { BugStats, BurndownPoint, Metric, Progress, VelocityPoint } from '@/api/types'
import { chartBase } from '@/lib/echarts'
import { useProjectStore } from '@/stores/project'
import { formatBeijing } from '@/utils/time'

const project = useProjectStore()
const metric = ref<Metric>('points')
const burn = ref<BurndownPoint[]>([])
const velocity = ref<VelocityPoint[]>([])
const progress = ref<Progress | null>(null)
const bugs = ref<BugStats | null>(null)
const loading = ref(false)

const metricLabel: Record<Metric, string> = { points: '故事点', hours: '预估工时', count: '任务数' }

async function reload() {
  if (!project.current) return
  loading.value = true
  try {
    const sprintId = project.activeSprint?.id
    const [v, b] = await Promise.all([
      getVelocity(project.current.id),
      getBugStats(project.current.id).then((r) => r.data).catch(() => null),
    ])
    velocity.value = v
    bugs.value = b
    if (sprintId) {
      burn.value = await getBurndown(sprintId, metric.value)
      progress.value = (await getProgress(sprintId)).data
    } else {
      burn.value = []
      progress.value = null
    }
  } finally {
    loading.value = false
  }
}

onMounted(reload)
watch(() => [project.current?.id, project.activeSprint?.id], reload)
watch(metric, async () => {
  if (project.activeSprint) burn.value = await getBurndown(project.activeSprint.id, metric.value)
})

const severityOption = computed(() => {
  const data = Object.entries(bugs.value?.by_severity || {}).map(([name, value]) => ({ name, value }))
  return {
    ...chartBase,
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item' as const, backgroundColor: '#1B2030', textStyle: { color: '#F3EBE0' } },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        data,
        color: ['#C45C5C', '#D46A2C', '#E9C46A', '#2A9D8F', '#C9BFAF'],
        label: { color: '#C9BFAF' },
      },
    ],
  }
})

const statusOption = computed(() => {
  const data = Object.entries(bugs.value?.by_status || {}).map(([name, value]) => ({ name, value }))
  return {
    ...chartBase,
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item' as const, backgroundColor: '#1B2030', textStyle: { color: '#F3EBE0' } },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        data,
        color: ['#C9BFAF', '#D46A2C', '#2A9D8F', '#E9C46A', '#7C9A6C', '#C45C5C'],
        label: { color: '#C9BFAF' },
      },
    ],
  }
})

const cards = computed(() => {
  const p = progress.value
  return [
    { label: '完成率', value: p ? `${Math.round(p.completion_rate * 100)}%` : '—' },
    { label: '剩余天数', value: p ? String(p.remaining_days) : '—' },
    { label: '平均速度', value: p ? `${p.velocity}` : '—' },
    { label: '预测完成', value: (p?.predicted_done_date || p?.predicted_end) ? formatBeijing(p.predicted_done_date || p.predicted_end || '') : '—' },
  ]
})
</script>

<template>
  <div>
    <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 class="font-display text-3xl">统计</h2>
        <p class="text-sm text-paper-dim">燃尽 · 生产力 · 进度 · Bug 分布</p>
      </div>
      <div class="flex rounded-lg bg-ink-3 p-1">
        <button
          v-for="m in (['points', 'hours', 'count'] as const)"
          :key="m"
          type="button"
          class="rounded-md px-3 py-1 text-sm"
          :class="metric === m ? 'bg-copper text-paper' : 'text-paper-dim'"
          @click="metric = m"
        >
          {{ metricLabel[m] }}
        </button>
      </div>
    </div>

    <p v-if="loading" class="mb-3 text-sm text-paper-dim">聚合统计中…</p>

    <div class="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="c in cards" :key="c.label" class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
        <p class="text-xs uppercase tracking-wider text-paper-dim">{{ c.label }}</p>
        <p class="mt-2 font-display text-2xl">{{ c.value }}</p>
      </article>
    </div>

    <div class="grid gap-4 xl:grid-cols-2">
      <section>
        <h3 class="mb-2 font-display text-xl">燃尽图 · {{ metricLabel[metric] }}</h3>
        <BurndownChart :points="burn" :metric="metric" />
      </section>
      <section>
        <h3 class="mb-2 font-display text-xl">团队生产力</h3>
        <VelocityChart :points="velocity" />
      </section>
      <section>
        <h3 class="mb-2 font-display text-xl">Bug 严重级别</h3>
        <div class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
          <v-chart v-if="bugs && Object.keys(bugs.by_severity || {}).length" class="h-64 w-full" :option="severityOption" autoresize />
          <div v-else class="grid h-64 place-items-center border border-dashed border-line text-sm text-paper-dim">暂无缺陷分布</div>
        </div>
      </section>
      <section>
        <h3 class="mb-2 font-display text-xl">Bug 状态 · MTTR {{ bugs?.mttr_hours ?? '—' }} h</h3>
        <div class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
          <v-chart v-if="bugs && Object.keys(bugs.by_status || {}).length" class="h-64 w-full" :option="statusOption" autoresize />
          <div v-else class="grid h-64 place-items-center border border-dashed border-line text-sm text-paper-dim">暂无状态分布</div>
        </div>
      </section>
    </div>
  </div>
</template>
