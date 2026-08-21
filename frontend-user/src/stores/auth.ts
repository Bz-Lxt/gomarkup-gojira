import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { fetchMe, login as loginApi } from '@/api/auth'
import { clearSession, persistSession } from '@/api/http'
import type { Role, User } from '@/api/types'
import { logger } from '@/utils/logger'

const USER_KEY = 'gojira.user'
const TOKEN_KEY = 'gojira.token'
const REFRESH_KEY = 'gojira.refresh'

function readUser(): User | null {
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as User) : null
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || '')
  const refreshToken = ref(localStorage.getItem(REFRESH_KEY) || '')
  const user = ref<User | null>(readUser())

  const isAuthed = computed(() => Boolean(token.value))
  const role = computed<Role | null>(() => user.value?.role ?? null)
  const displayName = computed(() => user.value?.display_name || user.value?.username || '访客')

  function applySession(payload: { token?: string; access_token?: string; refresh_token: string; user: User }) {
    const access = payload.token || payload.access_token || ''
    token.value = access
    refreshToken.value = payload.refresh_token
    user.value = payload.user
    persistSession(access, payload.refresh_token, JSON.stringify(payload.user))
  }

  async function login(username: string, password: string) {
    const res = await loginApi(username, password)
    applySession(res.data)
    return res.data.user
  }

  async function hydrate() {
    if (!token.value) return
    try {
      const res = await fetchMe()
      user.value = res.data
      persistSession(token.value, refreshToken.value, JSON.stringify(res.data))
    } catch (err) {
      logger.warn('hydrate me failed', err)
    }
  }

  function logout() {
    token.value = ''
    refreshToken.value = ''
    user.value = null
    clearSession()
  }

  return { token, refreshToken, user, isAuthed, role, displayName, login, logout, hydrate, applySession }
})
