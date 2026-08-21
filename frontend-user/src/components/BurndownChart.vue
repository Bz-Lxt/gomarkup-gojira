<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import type { BurndownPoint, Metric } from '@/api/types'
import { chartBase } from '@/lib/echarts'
import { formatBeijingDate } from '@/utils/time'

const props = defineProps<{ points: BurndownPoint[]; metric: Metric }>()

const unit = computed(() => (props.metric === 'hours' ? '小时' : props.metric === 'count' ? '任务' : '故事点'))

const option = computed(() => ({
  ...chartBase,
  backgroundColor: 'transparent',
  legend: { data: ['理想', '实际'], textStyle: { color: '#C9BFAF' }, top: 0 },
  xAxis: {
    type: 'category',
    data: props.points.map((p) => formatBeijingDate(p.date)),
    axisLine: { lineStyle: { color: 'rgba(243,235,224,0.16)' } },
    axisLabel: { color: '#C9BFAF' },
  },
  yAxis: {
    type: 'value',
    name: unit.value,
    axisLabel: { color: '#C9BFAF' },
    splitLine: { lineStyle: { color: 'rgba(243,235,224,0.06)' } },
  },
  series: [
    {
      name: '理想',
      type: 'line',
      data: props.points.map((p) => p.ideal),
      smooth: false,
      lineStyle: { type: 'dashed', color: '#C9BFAF', width: 1.5 },
      itemStyle: { color: '#C9BFAF' },
      symbol: 'none',
    },
    {
      name: '实际',
      type: 'line',
      data: props.points.map((p) => p.actual),
      smooth: true,
      lineStyle: { color: '#D46A2C', width: 2.4 },
      itemStyle: { color: '#D46A2C' },
      markPoint: {
        data: props.points
          .filter((p) => p.scope_change)
          .map((p) => ({
            name: '范围变更',
            coord: [formatBeijingDate(p.date), p.actual],
            value: `+${p.scope_change}`,
          })),
        itemStyle: { color: '#E9C46A' },
      },
    },
  ],
}))
</script>

<template>
  <div class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
    <v-chart v-if="points.length" class="h-72 w-full" :option="option" autoresize />
    <div v-else class="grid h-72 place-items-center border border-dashed border-line text-sm text-paper-dim">
      暂无燃尽数据
    </div>
  </div>
</template>
