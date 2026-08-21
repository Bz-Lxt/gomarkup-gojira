<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  addComment,
  createIssue,
  getIssue,
  listComments,
  listHistory,
  transitionIssue,
  updateIssue,
} from '@/api/issues'
import type { Comment, Issue, IssueDraft, StatusHistory } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useBoardStore } from '@/stores/board'
import { useProjectStore } from '@/stores/project'
import { useToastStore } from '@/stores/toast'
import { useUiStore } from '@/stores/ui'
import { formatBeijing } from '@/utils/time'
import { allowedTransitions, FIBONACCI, TYPE_LABEL } from '@/utils/workflow'

const emit = defineEmits<{ saved: [Issue] }>()

const ui = useUiStore()
const project = useProjectStore()
const board = useBoardStore()
const auth = useAuthStore()
const toast = useToastStore()

const loading = ref(false)
const saving = ref(false)
const comments = ref<Comment[]>([])
const history = ref<StatusHistory[]>([])
const commentBody = ref('')
const errors = reactive<Record<string, string>>({})
const issue = ref<Issue | null>(null)

const form = reactive<IssueDraft>({
  title: '',
  description: '',
  issue_type: 'TASK',
  priority: 'MEDIUM',
  severity: 'MAJOR',
  assignee_id: null,
  sprint_id: null,
  story_points: 3,
  estimate_hours: 0,
  start_date: '',
  due_date: '',
  labels: [],
  reproduce_steps: '',
  affected_version: '',
  fix_version: '',
})

const labelText = ref('')

const transitions = computed(() => (issue.value ? allowedTransitions(issue.value, project.myRole) : []))
const fullscreen = computed(() => (typeof window !== 'undefined' ? window.innerWidth < 768 : false))

function resetForm() {
  Object.assign(form, {
    title: '',
    description: '',
    issue_type: ui.draftType,
    priority: 'MEDIUM',
    severity: 'MAJOR',
    assignee_id: null,
    sprint_id: project.activeSprint?.id ?? null,
    story_points: 3,
    estimate_hours: 0,
    start_date: '',
    due_date: '',
    labels: [],
    reproduce_steps: '',
    affected_version: '',
    fix_version: '',
  })
  labelText.value = ''
  issue.value = null
  comments.value = []
  history.value = []
  Object.keys(errors).forEach((k) => delete errors[k])
}

function applyIssue(next: Issue) {
  issue.value = next
  form.title = next.title
  form.description = next.description || ''
  form.issue_type = next.issue_type
  form.priority = next.priority
  form.severity = next.severity || 'MAJOR'
  form.assignee_id = next.assignee_id ?? null
  form.sprint_id = next.sprint_id ?? null
  form.story_points = next.story_points ?? 0
  form.estimate_hours = next.estimate_hours ?? 0
  form.start_date = next.start_date ? next.start_date.slice(0, 10) : ''
  form.due_date = next.due_date ? next.due_date.slice(0, 10) : ''
  form.labels = next.labels || []
  form.reproduce_steps = next.reproduce_steps || ''
  form.affected_version = next.affected_version || ''
  form.fix_version = next.fix_version || ''
  labelText.value = (next.labels || []).join(', ')
}

async function loadDetail(id: string) {
  loading.value = true
  try {
    const res = await getIssue(id)
    applyIssue(res.data)
    comments.value = await listComments(id)
    history.value = await listHistory(id)
  } finally {
    loading.value = false
  }
}

watch(
  () => [ui.drawerOpen, ui.activeIssueId, ui.drawerMode] as const,
  ([open, id, mode]) => {
    if (!open) return
    if (mode === 'create' || !id) resetForm()
    else void loadDetail(id)
  },
  { immediate: true },
)

function validate(): boolean {
  Object.keys(errors).forEach((k) => delete errors[k])
  if (!form.title.trim()) errors.title = '标题不能为空'
  if (!FIBONACCI.includes(form.story_points as (typeof FIBONACCI)[number])) errors.story_points = '故事点须为斐波那契枚举'
  if (form.estimate_hours < 0) errors.estimate_hours = '预估工时不能为负'
  if (form.start_date && form.due_date && form.start_date > form.due_date) {
    errors.due_date = '截止日期不得早于开始日期'
  }
  if (form.issue_type === 'BUG') {
    if (!form.severity) errors.severity = '缺陷必须选择严重级别'
    if (!form.reproduce_steps?.trim()) errors.reproduce_steps = '请填写复现步骤'
  }
  if (Object.keys(errors).length) {
    toast.error('请先修正表单错误后再保存')
    return false
  }
  return true
}

function payload(): IssueDraft {
  return {
    ...form,
    title: form.title.trim(),
    labels: labelText.value
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean),
    start_date: form.start_date || null,
    due_date: form.due_date || null,
    assignee_id: form.assignee_id || null,
    sprint_id: form.sprint_id || null,
  }
}

async function save() {
  if (!validate() || !project.current) return
  saving.value = true
  try {
    if (ui.drawerMode === 'create') {
      const res = await createIssue(project.current.id, payload())
      board.prependIssue(res.data)
      toast.success('已创建事项')
      emit('saved', res.data)
      ui.closeDrawer()
    } else if (issue.value) {
      const res = await updateIssue(issue.value.id, { ...payload(), version: issue.value.version })
      board.replaceIssue(res.data)
      applyIssue(res.data)
      toast.success('已保存')
      emit('saved', res.data)
    }
  } finally {
    saving.value = false
  }
}

async function runTransition(to: string) {
  if (!issue.value) return
  const res = await transitionIssue(issue.value.id, to, issue.value.version)
  const body = res.data
  const next = body && typeof body === 'object' && 'issue' in body && body.issue ? body.issue : (body as Issue)
  if (next?.id) {
    board.replaceIssue(next)
    applyIssue(next)
    history.value = await listHistory(next.id)
  }
  const warnings = body && typeof body === 'object' && 'warnings' in body ? body.warnings : undefined
  if (warnings?.length) toast.warn(warnings.join('；'))
  else toast.success(`已转到 ${to}`)
}

async function sendComment() {
  if (!issue.value || !commentBody.value.trim()) return
  const res = await addComment(issue.value.id, commentBody.value.trim())
  comments.value.push(res.data)
  commentBody.value = ''
}
</script>

<template>
  <Teleport to="body">
    <div v-if="ui.drawerOpen" class="fixed inset-0 z-50 flex justify-end">
      <button class="absolute inset-0 bg-black/45" type="button" aria-label="关闭抽屉" @click="ui.closeDrawer" />
      <aside
        class="relative flex h-full w-full max-w-none flex-col bg-ink-2 shadow-paper transition-transform duration-drawer md:max-w-[480px]"
        :class="fullscreen ? 'translate-x-0' : ''"
      >
        <header class="flex items-center justify-between border-b border-line px-5 py-4">
          <div>
            <p class="font-mono text-xs text-paper-dim">
              {{ ui.drawerMode === 'create' ? 'NEW' : issue?.key || 'ISSUE' }}
            </p>
            <h2 class="font-display text-2xl">{{ ui.drawerMode === 'create' ? '创建事项' : '编辑事项' }}</h2>
          </div>
          <button type="button" class="btn-ghost rounded-lg px-3 py-1.5 text-sm" @click="ui.closeDrawer">关闭</button>
        </header>

        <div v-if="loading" class="px-5 py-8 text-sm text-paper-dim">正在读取…</div>
        <div v-else class="scrollbar-thin flex-1 overflow-y-auto px-5 py-4">
          <div class="grid gap-3">
            <label class="grid gap-1 text-sm">
              标题 *
              <input v-model="form.title" class="field" maxlength="160" />
              <span v-if="errors.title" class="field-error">{{ errors.title }}</span>
            </label>
            <div class="grid grid-cols-2 gap-3">
              <label class="grid gap-1 text-sm">
                类型
                <select v-model="form.issue_type" class="field" :disabled="ui.drawerMode === 'edit'">
                  <option value="STORY">故事</option>
                  <option value="TASK">任务</option>
                  <option value="BUG">缺陷</option>
                </select>
              </label>
              <label class="grid gap-1 text-sm">
                优先级
                <select v-model="form.priority" class="field">
                  <option value="BLOCKER">Blocker</option>
                  <option value="HIGHEST">Highest</option>
                  <option value="HIGH">High</option>
                  <option value="MEDIUM">Medium</option>
                  <option value="LOW">Low</option>
                  <option value="LOWEST">Lowest</option>
                </select>
              </label>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <label class="grid gap-1 text-sm">
                经办人
                <select v-model="form.assignee_id" class="field">
                  <option :value="null">未指派</option>
                  <option v-for="m in project.members" :key="m.user_id" :value="m.user_id">
                    {{ m.user?.display_name || m.display_name || m.username }}
                  </option>
                </select>
              </label>
              <label class="grid gap-1 text-sm">
                Sprint
                <select v-model="form.sprint_id" class="field">
                  <option :value="null">Backlog</option>
                  <option v-for="s in project.sprints" :key="s.id" :value="s.id">{{ s.name }}</option>
                </select>
              </label>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <label class="grid gap-1 text-sm">
                故事点
                <select v-model.number="form.story_points" class="field">
                  <option v-for="n in FIBONACCI" :key="n" :value="n">{{ n }}</option>
                </select>
                <span v-if="errors.story_points" class="field-error">{{ errors.story_points }}</span>
              </label>
              <label class="grid gap-1 text-sm">
                预估工时
                <input v-model.number="form.estimate_hours" class="field" type="number" min="0" step="0.5" />
                <span v-if="errors.estimate_hours" class="field-error">{{ errors.estimate_hours }}</span>
              </label>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <label class="grid gap-1 text-sm">
                开始日期
                <input v-model="form.start_date" class="field" type="date" />
              </label>
              <label class="grid gap-1 text-sm">
                截止日期
                <input v-model="form.due_date" class="field" type="date" />
                <span v-if="errors.due_date" class="field-error">{{ errors.due_date }}</span>
              </label>
            </div>
            <label class="grid gap-1 text-sm">
              标签（逗号分隔）
              <input v-model="labelText" class="field" />
            </label>
            <label class="grid gap-1 text-sm">
              描述
              <textarea v-model="form.description" class="field min-h-[96px]" />
            </label>
            <template v-if="form.issue_type === 'BUG'">
              <label class="grid gap-1 text-sm">
                严重级别 *
                <select v-model="form.severity" class="field">
                  <option value="BLOCKER">Blocker</option>
                  <option value="CRITICAL">Critical</option>
                  <option value="MAJOR">Major</option>
                  <option value="MINOR">Minor</option>
                  <option value="TRIVIAL">Trivial</option>
                </select>
                <span v-if="errors.severity" class="field-error">{{ errors.severity }}</span>
              </label>
              <label class="grid gap-1 text-sm">
                复现步骤 *
                <textarea v-model="form.reproduce_steps" class="field min-h-[80px]" />
                <span v-if="errors.reproduce_steps" class="field-error">{{ errors.reproduce_steps }}</span>
              </label>
              <div class="grid grid-cols-2 gap-3">
                <label class="grid gap-1 text-sm">影响版本<input v-model="form.affected_version" class="field" /></label>
                <label class="grid gap-1 text-sm">修复版本<input v-model="form.fix_version" class="field" /></label>
              </div>
            </template>
          </div>

          <div v-if="issue && transitions.length" class="mt-5">
            <p class="mb-2 text-xs uppercase tracking-wider text-paper-dim">按角色可转状态</p>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="t in transitions"
                :key="t.to"
                type="button"
                class="btn-ghost rounded-lg px-3 py-1.5 text-xs"
                @click="runTransition(t.to)"
              >
                {{ t.label }}
              </button>
            </div>
          </div>

          <section v-if="issue" class="mt-6">
            <h3 class="font-display text-lg">评论</h3>
            <ul class="mt-3 space-y-3">
              <li v-for="c in comments" :key="c.id" class="rounded-lg bg-ink-3 px-3 py-2">
                <p class="text-xs text-paper-dim">
                  {{ c.author?.display_name || '成员' }} · {{ formatBeijing(c.created_at) }}
                </p>
                <p class="mt-1 whitespace-pre-wrap text-sm">{{ c.body }}</p>
              </li>
              <li v-if="!comments.length" class="text-sm text-paper-dim">还没有评论。</li>
            </ul>
            <div class="mt-3 flex gap-2">
              <input v-model="commentBody" class="field" :placeholder="`以 ${auth.displayName} 身份留言`" />
              <button type="button" class="btn-primary shrink-0 rounded-lg px-3 text-sm" @click="sendComment">发送</button>
            </div>
          </section>

          <section v-if="history.length" class="mt-6">
            <h3 class="font-display text-lg">状态历史</h3>
            <ul class="mt-3 space-y-2 text-sm text-paper-dim">
              <li v-for="h in history" :key="h.id">
                {{ h.from_status }} → {{ h.to_status }} · {{ formatBeijing(h.changed_at) }}
              </li>
            </ul>
          </section>
        </div>

        <footer class="flex items-center justify-between border-t border-line px-5 py-4">
          <span class="text-xs text-paper-dim">{{ TYPE_LABEL[form.issue_type] }} · 北京时间</span>
          <button type="button" class="btn-primary rounded-lg px-4 py-2 text-sm disabled:opacity-50" :disabled="saving" @click="save">
            {{ saving ? '保存中…' : '保存' }}
          </button>
        </footer>
      </aside>
    </div>
  </Teleport>
</template>
