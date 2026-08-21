<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { listTriggerExecutions, listTriggers } from '@/api/triggers'
import type { Trigger, TriggerExecution } from '@/api/types'
import { useProjectStore } from '@/stores/project'
import { formatBeijing } from '@/utils/time'

const project = useProjectStore()
const triggers = ref<Trigger[]>([])
const logs = ref<TriggerExecution[]>([])
const loading = ref(false)

async function reload() {
  if (!project.current) return
  loading.value = true
  try {
    triggers.value = await listTriggers(project.current.id)
    logs.value = await listTriggerExecutions(project.current.id)
  } finally {
    loading.value = false
  }
}

onMounted(reload)
watch(() => project.current?.id, reload)

function statusTone(s: string) {
  if (s === 'SUCCESS' || s === 'OK') return 'text-olive'
  if (s === 'RETRY' || s === 'PENDING') return 'text-gold'
  return 'text-rose'
}
</script>

<template>
  <div>
    <div class="mb-5">
      <h2 class="font-display text-3xl">触发器</h2>
      <p class="text-sm text-paper-dim">只读列表与执行日志 · 可视化配置属 V2</p>
    </div>

    <p v-if="loading" class="text-sm text-paper-dim">读取触发器…</p>

    <section class="mb-6">
      <h3 class="mb-3 font-display text-xl">已配置触发器</h3>
      <div v-if="!triggers.length" class="grid h-32 place-items-center rounded-card border border-dashed border-line text-sm text-paper-dim">
        还没有触发器。种子规则会在任务进入「已完成」时通知 PM。
      </div>
      <ul v-else class="grid gap-3 md:grid-cols-2">
        <li v-for="t in triggers" :key="t.id" class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
          <div class="flex items-start justify-between gap-2">
            <h4 class="font-medium">{{ t.name }}</h4>
            <span class="rounded-full px-2 py-0.5 text-xs" :class="t.is_enabled ? 'bg-olive/20 text-olive' : 'bg-ink-3 text-paper-dim'">
              {{ t.is_enabled ? '启用' : '停用' }}
            </span>
          </div>
          <p class="mt-2 font-mono text-xs text-paper-dim">{{ t.event_type }}</p>
        </li>
      </ul>
    </section>

    <section>
      <h3 class="mb-3 font-display text-xl">执行日志</h3>
      <div class="overflow-x-auto rounded-card ring-1 ring-line">
        <table class="w-full min-w-[720px] text-left text-sm">
          <thead class="bg-ink-2 text-xs uppercase tracking-wider text-paper-dim">
            <tr>
              <th class="px-3 py-2">时间</th>
              <th class="px-3 py-2">触发器</th>
              <th class="px-3 py-2">动作</th>
              <th class="px-3 py-2">状态</th>
              <th class="px-3 py-2">重试</th>
              <th class="px-3 py-2">耗时</th>
              <th class="px-3 py-2">错误</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in logs" :key="row.id" class="border-t border-line">
              <td class="px-3 py-2 font-mono text-xs">{{ formatBeijing(row.created_at) }}</td>
              <td class="px-3 py-2">{{ row.trigger_name || row.trigger_id }}</td>
              <td class="px-3 py-2 font-mono text-xs">{{ row.action_type }}</td>
              <td class="px-3 py-2" :class="statusTone(row.status)">{{ row.status }}</td>
              <td class="px-3 py-2">{{ row.retry_count }}</td>
              <td class="px-3 py-2">{{ row.duration_ms }} ms</td>
              <td class="max-w-[240px] truncate px-3 py-2 text-paper-dim">{{ row.error_msg || '—' }}</td>
            </tr>
            <tr v-if="!logs.length">
              <td colspan="7" class="px-3 py-10 text-center text-paper-dim">暂无执行记录</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
