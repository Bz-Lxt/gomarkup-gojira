import { http } from './http'
import type { AuthPayload, User } from './types'

export function login(username: string, password: string) {
  return http.post<AuthPayload>('/v1/auth/login', { username, password })
}

export function register(payload: { username: string; password: string; email: string; display_name: string }) {
  return http.post<AuthPayload>('/v1/auth/register', payload)
}

export function fetchMe() {
  return http.get<User>('/v1/auth/me')
}

export function refreshToken(refresh_token: string) {
  return http.post<AuthPayload>('/v1/auth/refresh', { refresh_token })
}
