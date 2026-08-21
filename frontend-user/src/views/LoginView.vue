<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { SEED_ACCOUNTS } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { ROLE_LABEL } from '@/utils/workflow'

const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const route = useRoute()

const form = reactive({ username: 'admin', password: 'Admin@123' })
const errors = reactive<Record<string, string>>({})
const loading = ref(false)

function fill(username: string, password: string) {
  form.username = username
  form.password = password
}

function validate() {
  Object.keys(errors).forEach((k) => delete errors[k])
  if (!form.username.trim()) errors.username = '请输入用户名'
  if (!form.password) errors.password = '请输入密码'
  return Object.keys(errors).length === 0
}

async function submit() {
  if (!validate()) {
    toast.error('请先补全登录信息')
    return
  }
  loading.value = true
  try {
    await auth.login(form.username.trim(), form.password)
    toast.success(`欢迎回来，${auth.displayName}`)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/projects'
    await router.push(redirect)
  } catch {
    // interceptor already toasted
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center px-4 py-10">
    <div class="w-full max-w-md">
      <p class="mb-3 font-display text-sm tracking-[0.2em] text-copper">WORKSHOP DUSK</p>
      <h1 class="font-display text-4xl">GoJira</h1>
      <p class="mt-2 text-paper-dim">墨色工作台上的 Mini Jira</p>

      <form class="paper-card mt-8 p-6" @submit.prevent="submit">
        <label class="grid gap-1 text-sm text-ink">
          用户名
          <input v-model="form.username" class="field !bg-ink-3 !text-paper" autocomplete="username" />
          <span v-if="errors.username" class="field-error">{{ errors.username }}</span>
        </label>
        <label class="mt-4 grid gap-1 text-sm text-ink">
          密码
          <input v-model="form.password" class="field !bg-ink-3 !text-paper" type="password" autocomplete="current-password" />
          <span v-if="errors.password" class="field-error">{{ errors.password }}</span>
        </label>
        <button type="submit" class="btn-primary mt-6 w-full rounded-lg py-2.5 text-sm disabled:opacity-50" aria-label="登录" :disabled="loading">
          {{ loading ? '登录中…' : '进入工坊' }}
        </button>
      </form>

      <div class="mt-4 rounded-card border border-dashed border-line bg-ink-2/80 px-4 py-3">
        <p class="text-xs uppercase tracking-wider text-paper-dim">种子账号</p>
        <div class="mt-2 grid grid-cols-2 gap-2">
          <button
            v-for="acc in SEED_ACCOUNTS"
            :key="acc.username"
            type="button"
            class="rounded-lg bg-ink-3 px-3 py-2 text-left text-xs hover:bg-ink-3/80"
            @click="fill(acc.username, acc.password)"
          >
            <span class="font-mono text-paper">{{ acc.username }}</span>
            <span class="block text-paper-dim">{{ ROLE_LABEL[acc.role] }} / {{ acc.password }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
