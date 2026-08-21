import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { ApiError, type ApiErrorBody, type Envelope } from './types'
import { logger } from '@/utils/logger'

const TOKEN_KEY = 'gojira.token'
const REFRESH_KEY = 'gojira.refresh'
const USER_KEY = 'gojira.user'

export function readToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function persistSession(token: string, refresh: string, userJson: string) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(REFRESH_KEY, refresh)
  localStorage.setItem(USER_KEY, userJson)
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_KEY)
  localStorage.removeItem(USER_KEY)
}

let onUnauthorized: (() => void) | null = null
let onApiError: ((err: ApiError) => void) | null = null

export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

export function setApiErrorHandler(fn: (err: ApiError) => void) {
  onApiError = fn
}

export const http = axios.create({
  baseURL: '/api',
  timeout: 20000,
})

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = readToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

let refreshing: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  const refresh = localStorage.getItem(REFRESH_KEY)
  if (!refresh) return false
  try {
    const res = await axios.post<Envelope<{ token?: string; access_token?: string; refresh_token?: string }>>(
      '/api/v1/auth/refresh',
      { refresh_token: refresh },
    )
    const payload = res.data?.data ?? (res.data as unknown as { token?: string; access_token?: string })
    const token = payload.token || payload.access_token
    if (!token) return false
    localStorage.setItem(TOKEN_KEY, token)
    if (payload.refresh_token) localStorage.setItem(REFRESH_KEY, payload.refresh_token)
    return true
  } catch (err) {
    logger.warn('refresh failed', err)
    return false
  }
}

http.interceptors.response.use(
  (res) => {
    const body = res.data
    if (body && typeof body === 'object' && 'data' in body) {
      res.data = (body as Envelope<unknown>).data
      ;(res as typeof res & { meta?: unknown }).meta = (body as Envelope<unknown>).meta
    }
    return res
  },
  async (error: AxiosError<ApiErrorBody>) => {
    const status = error.response?.status ?? 0
    const cfg = error.config as (InternalAxiosRequestConfig & { _retry?: boolean }) | undefined

    if (status === 401 && cfg && !cfg._retry && !cfg.url?.includes('/auth/login')) {
      cfg._retry = true
      if (!refreshing) refreshing = tryRefresh().finally(() => { refreshing = null })
      const ok = await refreshing
      if (ok) return http(cfg)
      clearSession()
      onUnauthorized?.()
    }

    const body = error.response?.data ?? {}
    const apiErr = new ApiError(status, body)
    if (status !== 401) onApiError?.(apiErr)
    return Promise.reject(apiErr)
  },
)

export function unwrapList<T>(raw: unknown): T[] {
  if (Array.isArray(raw)) return raw as T[]
  if (raw && typeof raw === 'object') {
    const obj = raw as Record<string, unknown>
    for (const key of ['items', 'list', 'projects', 'issues', 'sprints', 'members', 'comments', 'triggers', 'executions']) {
      if (Array.isArray(obj[key])) return obj[key] as T[]
    }
  }
  return []
}
