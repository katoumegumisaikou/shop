import Taro from '@tarojs/taro'

const TOKEN_KEY = 'access_token'

function getToken(): string | null {
  try {
    return Taro.getStorageSync(TOKEN_KEY) || null
  } catch {
    return null
  }
}

export function setToken(token: string) {
  Taro.setStorageSync(TOKEN_KEY, token)
}

export function removeToken() {
  try {
    Taro.removeStorageSync(TOKEN_KEY)
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
    // 401 未授权，清除 token 并跳转登录
    if (body.code === 401) {
      removeToken()
      Taro.redirectTo({ url: '/pages/auth/login/index' })
      throw new Error('未登录')
    }
    throw new Error(body.message || '请求失败')
  }

  return body.data
}
