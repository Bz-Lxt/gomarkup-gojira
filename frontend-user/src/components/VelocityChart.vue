<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import type { VelocityPoint } from '@/api/types'
import { chartBase } from '@/lib/echarts'

const props = defineProps<{ points: VelocityPoint[] }>()

const option = computed(() => {
  const weeks = [...new Set(props.points.map((p) => p.week))]
  const names = [...new Set(props.points.map((p) => p.display_name || p.user_id || '成员'))]
  const palette = ['#D46A2C', '#2A9D8F', '#E9C46A', '#7C9A6C', '#C45C5C', '#C9BFAF']
  return {
    ...chartBase,
    backgroundColor: 'transparent',
    legend: { data: names, textStyle: { color: '#C9BFAF' }, top: 0 },
    xAxis: {
      type: 'category',
      data: weeks,
      axisLabel: { color: '#C9BFAF' },
      axisLine: { lineStyle: { color: 'rgba(243,235,224,0.16)' } },
    },
    yAxis: {
      type: 'value',
      name: '故事点',
      axisLabel: { color: '#C9BFAF' },
      splitLine: { lineStyle: { color: 'rgba(243,235,224,0.06)' } },
    },
    series: names.map((name, idx) => ({
      name,
      type: 'line',
      data: weeks.map((w) => {
        const hit = props.points.find((p) => p.week === w && (p.display_name || p.user_id) === name)
        return hit?.points ?? 0
      }),
      lineStyle: { width: 2, color: palette[idx % palette.length] },
      itemStyle: { color: palette[idx % palette.length] },
    })),
  }
})
</script>

<template>
  <div class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
    <v-chart v-if="points.length" class="h-72 w-full" :option="option" autoresize />
    <div v-else class="grid h-72 place-items-center border border-dashed border-line text-sm text-paper-dim">
      暂无生产力数据
    </div>
  </div>
</template>
