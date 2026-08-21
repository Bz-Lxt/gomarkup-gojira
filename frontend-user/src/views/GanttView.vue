<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import GanttChart from '@/components/GanttChart.vue'
import { getGantt } from '@/api/gantt'
import { listIssues } from '@/api/issues'
import type { Dependency, GanttBar, Issue } from '@/api/types'
import { useProjectStore } from '@/stores/project'

const project = useProjectStore()
const zoom = ref<'day' | 'week' | 'month'>('week')
const bars = ref<GanttBar[]>([])
const issues = ref<Issue[]>([])
const deps = ref<Dependency[]>([])
const loading = ref(false)

async function reload() {
  if (!project.current) return
  loading.value = true
  try {
    const res = await getGantt(project.current.id)
    bars.value = res.data.bars || []
    deps.value = res.data.dependencies || []
    issues.value = res.data.issues || []
    if (!bars.value.length && !issues.value.length) {
      issues.value = await listIssues(project.current.id)
    }
  } catch {
    issues.value = await listIssues(project.current.id)
  } finally {
    loading.value = false
  }
}

onMounted(reload)
watch(() => project.current?.id, reload)
</script>

<template>
  <div>
    <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 class="font-display text-3xl">甘特图</h2>
        <p class="text-sm text-paper-dim">自研 SVG · FS 依赖贝塞尔 · 今日铜线 · 逾期玫瑰</p>
      </div>
      <div class="flex rounded-lg bg-ink-3 p-1">
        <button
          v-for="z in (['day', 'week', 'month'] as const)"
          :key="z"
          type="button"
          class="rounded-md px-3 py-1 text-sm"
          :class="zoom === z ? 'bg-copper text-paper' : 'text-paper-dim'"
          @click="zoom = z"
        >
          {{ z === 'day' ? '日' : z === 'week' ? '周' : '月' }}
        </button>
      </div>
    </div>
    <p v-if="loading" class="mb-3 text-sm text-paper-dim">绘制时间轴…</p>
    <GanttChart :bars="bars" :issues="issues" :dependencies="deps" :sprint="project.activeSprint" :zoom="zoom" />
  </div>
</template>
