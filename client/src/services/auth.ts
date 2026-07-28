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
  user: User
}

export interface RegisterResult {
  user: User
}

export interface SendSmsCode {
  code: string
  expiresIn: number
}

export async function phoneLogin(phone: string, password: string) {
  return request<LoginResult>('/c/auth/phone-login', {
    method: 'POST',
    data: { phone, password },
  })
}

export async function sendSmsCode(phone: string, purpose: string) {
  return request<SendSmsCode>('/c/auth/sms/code', {
    method: 'POST',
    data: { phone, purpose }
  })
}

export async function resetPassword(phone: string, code: string, password: string) {
  return request<void>('/c/auth/reset-password', {
    method: 'POST',
    auth: true,
    data: { phone, code, password }
  })
}

export async function getMe() {
  return request<User | null>('/c/me', { auth: true })
}
