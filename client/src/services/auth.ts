import { request } from './api'

export interface User {
  id: string
  phone: string | null
  nickname: string | null
  avatar: string | null
  gender: number
  status: string
  balance_cents: number
  created_at: string
}

export interface H5LoginResult {
  user: User
}

export interface H5RegisterResult {
  user: User
}

export interface MpLoginResult {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  refresh_expires_in: number
  user_id: string
}

export interface SendSmsCode {
  code: string
  expires_in: number
}

export async function MpLogin(code: string) {
  return request<MpLoginResult>('/c/auth/mp-login', {
    method: 'POST',
    data: { code },
  })
}

export async function PhoneLogin(phone: string, password: string) {
  return request<H5LoginResult>('/c/auth/phone-login', {
    method: 'POST',
    data: { phone, password },
  })
}

export async function RegisterByPhone(phone: string, password: string, code: string) {
  return request<H5RegisterResult>('/c/auth/phone-register', {
    method: 'POST',
    data: { phone, password, code }
  })
}

export interface RefreshTokenResult {
  access_token: string
  refresh_token: string
  expires_in: number
  refresh_expires_in: number
}

export async function RefreshToken(refreshToken: string) {
  return request<RefreshTokenResult>('/c/auth/refresh', {
    method: 'POST',
    data: { refresh_token: refreshToken },
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
