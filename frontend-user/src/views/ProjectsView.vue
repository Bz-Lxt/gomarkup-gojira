<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createProject } from '@/api/projects'
import { useAuthStore } from '@/stores/auth'
import { useProjectStore } from '@/stores/project'
import { useToastStore } from '@/stores/toast'
import { formatBeijing } from '@/utils/time'
import { ROLE_COLOR, ROLE_LABEL } from '@/utils/workflow'

const project = useProjectStore()
const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()

const showCreate = ref(false)
const form = reactive({ key: '', name: '', description: '' })
const errors = reactive<Record<string, string>>({})
const canCreate = ['ADMIN', 'PM'].includes(auth.role || '')

onMounted(() => {
  void project.loadAll()
})

function open(p: { key: string }) {
  void router.push(`/p/${p.key}/board`)
}

function validate() {
  Object.keys(errors).forEach((k) => delete errors[k])
  if (!/^[A-Z][A-Z0-9]{1,9}$/.test(form.key.trim().toUpperCase())) errors.key = 'Key 须为 2–10 位大写字母数字'
  if (!form.name.trim()) errors.name = '请填写项目名'
  return Object.keys(errors).length === 0
}

async function submit() {
  if (!validate()) {
    toast.error('请先修正表单')
    return
  }
  const res = await createProject({
    key: form.key.trim().toUpperCase(),
    name: form.name.trim(),
    description: form.description.trim(),
  })
  toast.success('项目已创建')
  showCreate.value = false
  await project.loadAll()
  void router.push(`/p/${res.data.key}/board`)
}

function logout() {
  auth.logout()
  void router.push('/login')
}
</script>

<template>
  <div class="min-h-screen bg-ink text-paper">
    <header class="flex h-14 items-center justify-between border-b border-line bg-ink-2 px-6">
      <h1 class="font-display text-xl">GoJira 项目</h1>
      <div class="flex items-center gap-3">
        <span class="flex items-center gap-2 text-sm">
          <i class="h-2.5 w-2.5 rounded-full" :style="{ background: ROLE_COLOR[auth.role || 'VIEWER'] }" />
          {{ auth.displayName }} · {{ ROLE_LABEL[auth.role || 'VIEWER'] }}
        </span>
        <button type="button" class="btn-ghost rounded-lg px-3 py-1.5 text-sm" @click="logout">退出</button>
      </div>
    </header>

    <main class="page-enter w-full px-6 py-8">
      <div class="mb-6 flex items-end justify-between">
        <div>
          <p class="text-xs tracking-[0.2em] text-copper">WORKSHOP</p>
          <h2 class="font-display text-3xl">选择一块工作台</h2>
        </div>
        <button v-if="canCreate" type="button" class="btn-primary rounded-lg px-4 py-2 text-sm" @click="showCreate = true">
          新建项目
        </button>
      </div>

      <div v-if="project.loading" class="text-paper-dim">正在读取项目…</div>
      <div v-else-if="!project.projects.length" class="grid h-56 place-items-center rounded-card border border-dashed border-line text-paper-dim">
        还没有项目。{{ canCreate ? '创建一个开始规划。' : '请联系管理员邀请你加入。' }}
      </div>
      <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <button
          v-for="p in project.projects"
          :key="p.id"
          type="button"
          class="paper-card p-5 text-left transition hover:-translate-y-0.5"
          @click="open(p)"
        >
          <p class="font-display text-2xl tracking-wide text-ink">{{ p.key }}</p>
          <h3 class="mt-1 text-lg font-medium text-ink">{{ p.name }}</h3>
          <p class="mt-2 line-clamp-2 text-sm text-ink/60">{{ p.description || '暂无描述' }}</p>
          <div class="mt-4 flex gap-4 text-xs text-ink/50">
            <span>{{ p.issue_count ?? '—' }} 事项</span>
            <span>{{ p.member_count ?? '—' }} 成员</span>
            <span>{{ formatBeijing(p.created_at) }}</span>
          </div>
        </button>
      </div>
    </main>

    <div v-if="showCreate" class="fixed inset-0 z-40 grid place-items-center bg-black/50 px-4" @click.self="showCreate = false">
      <form class="w-full max-w-md rounded-card bg-ink-2 p-6 ring-1 ring-line" @submit.prevent="submit">
        <h3 class="font-display text-2xl">新建项目</h3>
        <label class="mt-4 grid gap-1 text-sm">Key *<input v-model="form.key" class="field uppercase" /><span v-if="errors.key" class="field-error">{{ errors.key }}</span></label>
        <label class="mt-3 grid gap-1 text-sm">名称 *<input v-model="form.name" class="field" /><span v-if="errors.name" class="field-error">{{ errors.name }}</span></label>
        <label class="mt-3 grid gap-1 text-sm">描述<textarea v-model="form.description" class="field min-h-[80px]" /></label>
        <div class="mt-5 flex justify-end gap-2">
          <button type="button" class="btn-ghost rounded-lg px-4 py-2 text-sm" @click="showCreate = false">取消</button>
          <button type="submit" class="btn-primary rounded-lg px-4 py-2 text-sm">创建</button>
        </div>
      </form>
    </div>
  </div>
</template>
