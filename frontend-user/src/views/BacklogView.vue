<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import draggable from 'vuedraggable'
import IssueCard from '@/components/IssueCard.vue'
import { listIssues, updateIssue } from '@/api/issues'
import { addIssueToSprint, closeSprint, createSprint, removeIssueFromSprint, startSprint } from '@/api/sprints'
import type { Issue } from '@/api/types'
import { useProjectStore } from '@/stores/project'
import { useToastStore } from '@/stores/toast'
import { useUiStore } from '@/stores/ui'
import { formatBeijingDate } from '@/utils/time'

const project = useProjectStore()
const toast = useToastStore()
const ui = useUiStore()

const pool = ref<Issue[]>([])
const sprintIssues = ref<Issue[]>([])
const loading = ref(false)
const showCreate = ref(false)
const form = reactive({ name: '', goal: '', start_date: '', end_date: '' })
const errors = reactive<Record<string, string>>({})

const canPlan = computed(() => ['ADMIN', 'PM'].includes(project.myRole || ''))
const sprint = computed(() => project.activeSprint || project.sprints.find((s) => s.status === 'PLANNED') || null)

async function reload() {
  if (!project.current) return
  loading.value = true
  try {
    await project.loadSprints(project.current.id)
    const all = await listIssues(project.current.id)
    const sid = sprint.value?.id
    pool.value = all.filter((i) => !i.sprint_id)
    sprintIssues.value = sid ? all.filter((i) => i.sprint_id === sid) : []
  } finally {
    loading.value = false
  }
}

onMounted(reload)
watch(() => project.current?.id, reload)

async function onPoolChange(ev: { added?: { element: Issue } }) {
  if (!ev.added || !sprint.value) return
  const issue = ev.added.element
  try {
    await removeIssueFromSprint(sprint.value.id, issue.id)
    await updateIssue(issue.id, { sprint_id: null, version: issue.version })
    toast.success('已退回 Backlog')
  } catch {
    await reload()
  }
}

async function onSprintChange(ev: { added?: { element: Issue } }) {
  if (!ev.added) return
  if (!sprint.value) {
    toast.warn('请先创建或选择一个 Sprint')
    await reload()
    return
  }
  const issue = ev.added.element
  try {
    await addIssueToSprint(sprint.value.id, issue.id)
    toast.success('已入列当前 Sprint')
  } catch {
    await reload()
  }
}

function validateSprint() {
  Object.keys(errors).forEach((k) => delete errors[k])
  if (!form.name.trim()) errors.name = '请填写名称'
  if (!form.start_date || !form.end_date) errors.end_date = '请选择起止日期'
  if (form.start_date && form.end_date && form.start_date > form.end_date) errors.end_date = '结束不得早于开始'
  return Object.keys(errors).length === 0
}

async function makeSprint() {
  if (!project.current || !validateSprint()) {
    toast.error('请先修正 Sprint 表单')
    return
  }
  await createSprint(project.current.id, { ...form, name: form.name.trim(), goal: form.goal.trim() })
  toast.success('Sprint 已创建')
  showCreate.value = false
  await reload()
}

async function start() {
  if (!sprint.value) return
  const ok = await ui.ask('启动 Sprint', `将启动「${sprint.value.name}」并快照承诺范围。同项目仅允许一个 ACTIVE Sprint。`)
  if (!ok) return
  await startSprint(sprint.value.id)
  toast.success('Sprint 已启动')
  await reload()
}

async function close() {
  if (!sprint.value) return
  const ok = await ui.ask('关闭 Sprint', '未完成事项将退回 Backlog。')
  if (!ok) return
  await closeSprint(sprint.value.id, { move_to: 'backlog' })
  toast.success('Sprint 已关闭')
  await reload()
}
</script>

<template>
  <div>
    <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 class="font-display text-3xl">Sprint / Backlog</h2>
        <p class="text-sm text-paper-dim">未入列池与当前迭代 · 拖入 / 拖出即调用 API</p>
      </div>
      <div class="flex gap-2">
        <button v-if="canPlan" type="button" class="btn-ghost rounded-lg px-3 py-1.5 text-sm" @click="showCreate = true">
          新建 Sprint
        </button>
        <button
          v-if="canPlan && sprint?.status === 'PLANNED'"
          type="button"
          class="btn-primary rounded-lg px-3 py-1.5 text-sm"
          @click="start"
        >
          启动
        </button>
        <button
          v-if="canPlan && sprint?.status === 'ACTIVE'"
          type="button"
          class="btn-danger rounded-lg px-3 py-1.5 text-sm"
          @click="close"
        >
          关闭
        </button>
      </div>
    </div>

    <div v-if="loading" class="text-sm text-paper-dim">读取 Backlog…</div>
    <div v-else class="grid gap-4 lg:grid-cols-2">
      <section class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
        <header class="mb-3">
          <h3 class="font-display text-xl">未入列池</h3>
          <p class="text-xs text-paper-dim">{{ pool.length }} 项</p>
        </header>
        <draggable
          v-model="pool"
          group="sprint"
          item-key="id"
          :animation="160"
          ghost-class="drag-ghost"
          class="flex min-h-[280px] flex-col gap-2"
          @change="onPoolChange"
        >
          <template #item="{ element }">
            <IssueCard :issue="element" @click="ui.openIssue(element)" />
          </template>
          <template #footer>
            <div v-if="!pool.length" class="grid flex-1 place-items-center rounded-lg border border-dashed border-line py-10 text-sm text-paper-dim">
              池是空的，去创建事项或把卡片拖回来
            </div>
          </template>
        </draggable>
      </section>

      <section class="rounded-card bg-ink-2 p-4 ring-1 ring-line">
        <header class="mb-3">
          <h3 class="font-display text-xl">{{ sprint?.name || '当前 Sprint' }}</h3>
          <p class="text-xs text-paper-dim">
            <template v-if="sprint">
              {{ sprint.status }} · {{ formatBeijingDate(sprint.start_date) }} → {{ formatBeijingDate(sprint.end_date) }}
              · {{ sprintIssues.length }} 项
            </template>
            <template v-else>还没有可规划的迭代</template>
          </p>
        </header>
        <draggable
          v-model="sprintIssues"
          group="sprint"
          item-key="id"
          :animation="160"
          ghost-class="drag-ghost"
          class="flex min-h-[280px] flex-col gap-2"
          @change="onSprintChange"
        >
          <template #item="{ element }">
            <IssueCard :issue="element" @click="ui.openIssue(element)" />
          </template>
          <template #footer>
            <div v-if="!sprintIssues.length" class="grid flex-1 place-items-center rounded-lg border border-dashed border-line py-10 text-sm text-paper-dim">
              从左侧拖入故事与任务
            </div>
          </template>
        </draggable>
      </section>
    </div>

    <div v-if="showCreate" class="fixed inset-0 z-40 grid place-items-center bg-black/50 px-4" @click.self="showCreate = false">
      <form class="w-full max-w-md rounded-card bg-ink-2 p-6" @submit.prevent="makeSprint">
        <h3 class="font-display text-2xl">新建 Sprint</h3>
        <label class="mt-4 grid gap-1 text-sm">名称 *<input v-model="form.name" class="field" /><span v-if="errors.name" class="field-error">{{ errors.name }}</span></label>
        <label class="mt-3 grid gap-1 text-sm">目标<textarea v-model="form.goal" class="field min-h-[72px]" /></label>
        <div class="mt-3 grid grid-cols-2 gap-3">
          <label class="grid gap-1 text-sm">开始<input v-model="form.start_date" class="field" type="date" /></label>
          <label class="grid gap-1 text-sm">结束<input v-model="form.end_date" class="field" type="date" /><span v-if="errors.end_date" class="field-error">{{ errors.end_date }}</span></label>
        </div>
        <div class="mt-5 flex justify-end gap-2">
          <button type="button" class="btn-ghost rounded-lg px-4 py-2 text-sm" @click="showCreate = false">取消</button>
          <button type="submit" class="btn-primary rounded-lg px-4 py-2 text-sm">创建</button>
        </div>
      </form>
    </div>
  </div>
</template>
