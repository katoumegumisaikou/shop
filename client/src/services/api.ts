import { isH5 } from '@/utils/env'
import Taro from '@tarojs/taro'

const TOKEN_KEY = 'access_token'
const REFRESH_TOKEN_KEY = 'refresh_token'

function getToken(): string | null {
  try {
    return Taro.getStorageSync(TOKEN_KEY) || null
  } catch {
    return null
  }
}

function getRefreshToken(): string | null {
  try {
    return Taro.getStorageSync(REFRESH_TOKEN_KEY) || null
  } catch {
    return null
  }
}

// 是否正在刷新 token，避免并发请求同时刷新
let isRefreshing = false

export function setToken(token: string) {
  Taro.setStorageSync(TOKEN_KEY, token)
}

export function setRefreshToken(token: string) {
  Taro.setStorageSync(REFRESH_TOKEN_KEY, token)
}

export function removeToken() {
  try {
    Taro.removeStorageSync(TOKEN_KEY)
    Taro.removeStorageSync(REFRESH_TOKEN_KEY)
  } catch {
    // 忽略清理失败
  }
}

export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  header?: Record<string, string>
  auth?: boolean
  params?: Record<string, string | number | undefined>
  data?: unknown
}

const BASE_URL = process.env.TARO_APP_ID || 'http://localhost:8080/api/v1'

function buildURL(path: string, params?: Record<string, string | number | undefined>): string {
  if (!params) return `${BASE_URL}${path}`

  const parts: string[] = []
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
    }
  }

  if (parts.length === 0) return `${BASE_URL}${path}`
  return `${BASE_URL}${path}?${parts.join('&')}`
}

async function refreshToken(): Promise<boolean> {
  // 防并发的多个请求同时刷新
  if (isRefreshing) return false
  isRefreshing = true

  try {
    if (isH5()) {
      // H5：refresh_token 在 cookie 中，后端自动读取并更新 cookie
      await Taro.request({
        url: buildURL('/c/auth/refresh'),
        method: 'POST',
        header: { 'Content-Type': 'application/json' },
      })
      return true
    } else {
      // 小程序：手动读取 refresh_token，调接口换新 token
      const refreshTokenStr = getRefreshToken()
      if (!refreshTokenStr) return false

      const res = await Taro.request<ApiResponse<{
        access_token: string
        refresh_token: string
        expires_in: number
        refresh_expires_in: number
      }>>({
        url: buildURL('/c/auth/refresh'),
        method: 'POST',
        header: { 'Content-Type': 'application/json' },
        data: JSON.stringify({ refresh_token: refreshTokenStr }),
      })

      const body = res.data
      if (body.code !== 0) return false

      setToken(body.data.access_token)
      setRefreshToken(body.data.refresh_token)
      return true
    }
  } catch {
    return false
  } finally {
    isRefreshing = false
  }
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', data, auth = false, params, header = {} } = options

  header['Content-Type'] = 'application/json'

  if (auth) {
    const token = getToken()
    if (!token) {
      // 需要登录但没有 token，跳转到登录页
      Taro.redirectTo({ url: '/pages/auth/login/index' })
      throw new Error('未登录')
    }
    header['Authorization'] = `Bearer ${token}`
  }

  let res: Taro.request.SuccessCallbackResult<ApiResponse<T>>
  try {
    res = await Taro.request<ApiResponse<T>>({
      url: buildURL(path, params),
      method,
      data: method === 'GET' ? undefined : JSON.stringify(data),
      header,
    })
  } catch {
    throw new Error('网络异常，请检查网络连接后重试')
  }

  const body = res.data

  if (body.code !== 0) {
    // 401：尝试刷新 token 后重试一次
    if (body.code === 401) {
      if (await refreshToken()) {
        // 刷新成功，用新 token 重试原请求
        if (auth) {
          const newToken = getToken()
          if (newToken) {
            header['Authorization'] = `Bearer ${newToken}`
          }
        }
        try {
          res = await Taro.request<ApiResponse<T>>({
            url: buildURL(path, params),
            method,
            data: method === 'GET' ? undefined : JSON.stringify(data),
            header,
          })
          const retryBody = res.data
          if (retryBody.code === 0) return retryBody.data
        } catch {
          throw new Error('网络异常，请检查网络连接后重试')
        }
      }
      removeToken()
      Taro.redirectTo({ url: '/pages/auth/login/index' })
      throw new Error('未登录')
    }
    throw new Error(body.message || '请求失败')
  }

  return body.data
}
