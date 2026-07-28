import { request } from './api'

export interface User {
  id: string
  nickname: string | null
  avatar: string | null
  gender: number
  status: string
  balance_cents: number
  created_at: string
}

export interface LoginResult {
  access_token: string
  refresh_token: string
  expires_in: number
  refresh_expires_in: number
  user: User
}

export function login(phone: string, password: string) {
  return request<LoginResult>('/c/auth/phone-login', {
    method: 'POST',
    data: { phone, password },
  })
}

export function getMe() {
  return request<User | null>('/c/me', { auth: true })
}
