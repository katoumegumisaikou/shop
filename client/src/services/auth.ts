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
  expires_in: number
}

export async function PhoneLogin(phone: string, password: string) {
  return request<LoginResult>('/c/auth/phone-login', {
    method: 'POST',
    data: { phone, password },
  })
}

export async function RegisterByPhone(phone: string, password: string, code: string) {
  return request<RegisterResult>('/c/auth/phone-register', {
    method: 'POST',
    data: { phone, password, code }
  })
}

export async function SendSmsCode(phone: string, purpose: string) {
  return request<SendSmsCode>('/c/auth/sms/code', {
    method: 'POST',
    data: { phone, purpose }
  })
}

export async function ResetPassword(phone: string, code: string, password: string) {
  return request<void>('/c/auth/reset-password', {
    method: 'POST',
    auth: true,
    data: { phone, code, password }
  })
}

export async function GetMe() {
  return request<User | null>('/c/me', { auth: true })
}
