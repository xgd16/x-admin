/**
 * 统一 HTTP 客户端：注入令牌、处理 401 刷新、封装错误与文件上传/下载。
 */
import type { Envelope } from './types'
import { tokenStore } from './tokens'

const BASE = import.meta.env.VITE_API_BASE ?? '/admin/v1'

export class ApiError extends Error {
  code: number
  constructor(code: number, message: string) {
    super(message)
    this.code = code
  }
}

// 单飞刷新：多个请求同时 401 时只发起一次 refresh，其余请求复用同一个 Promise。
let refreshing: Promise<boolean> | null = null

// 认证接口自身不参与自动刷新，避免 401 循环。
const noRefreshPaths = new Set([
  '/auth/login',
  '/auth/login/mfa',
  '/auth/passkey/login/begin',
  '/auth/passkey/login/finish',
  '/auth/refresh',
])

/** 使用 refresh_token 换取新的令牌对；失败时返回 false，由调用方清理登录态。 */
async function refreshToken(): Promise<boolean> {
  if (!tokenStore.refresh) return false
  try {
    const res = await fetch(`${BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: tokenStore.refresh }),
    })
    if (!res.ok) return false
    const body = (await res.json()) as Envelope<{ access_token: string; refresh_token: string }>
    if (body.code !== 0) return false
    tokenStore.set(body.data.access_token, body.data.refresh_token)
    return true
  } catch {
    return false
  }
}

/** 把查询参数拼成 query string，自动忽略 undefined / null / 空字符串。 */
function queryString(params?: Record<string, unknown>): string {
  if (!params) return ''
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return
    search.set(key, String(value))
  })
  const text = search.toString()
  return text ? `?${text}` : ''
}

/** 解析统一响应包裹；HTTP 状态或业务 code 非 0 时抛出 ApiError。 */
async function parse<T>(res: Response): Promise<T> {
  const body = (await res.json()) as Envelope<T>
  if (!res.ok || body.code !== 0) {
    throw new ApiError(body.code || res.status, body.message || '请求失败')
  }
  return body.data
}

/**
 * 发送请求并在 401 时自动刷新令牌。
 * init 必须是可重复执行的工厂函数：刷新后需要重新读取最新 token 并重建请求体。
 */
async function send(
  path: string,
  params: Record<string, unknown> | undefined,
  init: () => RequestInit,
): Promise<Response> {
  const url = `${BASE}${path}${queryString(params)}`
  let res = await fetch(url, init())

  if (res.status === 401 && tokenStore.refresh && !noRefreshPaths.has(path)) {
    refreshing ??= refreshToken().finally(() => { refreshing = null })
    const ok = await refreshing
    if (!ok) {
      tokenStore.clear()
      window.location.assign('/login')
      throw new ApiError(40102, '登录已过期')
    }
    res = await fetch(url, init())
  }
  return res
}

/**
 * 发起 JSON API 请求。
 * 自动注入 Bearer Token；遇到 401 时先尝试刷新令牌，再重放原请求。
 */
async function request<T>(method: string, path: string, body?: unknown, params?: Record<string, unknown>): Promise<T> {
  const res = await send(path, params, () => {
    const headers: Record<string, string> = {}
    if (body !== undefined && !(body instanceof FormData)) headers['Content-Type'] = 'application/json'
    if (tokenStore.access) headers.Authorization = `Bearer ${tokenStore.access}`
    return {
      method,
      headers,
      body: body instanceof FormData ? body : body === undefined ? undefined : JSON.stringify(body),
    }
  })
  return parse<T>(res)
}

/** 业务代码统一通过这个对象访问后端，避免各处重复处理令牌和错误。 */
export const api = {
  get<T>(path: string, params?: Record<string, unknown>) {
    return request<T>('GET', path, undefined, params)
  },
  post<T>(path: string, body?: unknown) {
    return request<T>('POST', path, body)
  },
  put<T>(path: string, body?: unknown) {
    return request<T>('PUT', path, body)
  },
  del<T>(path: string, body?: unknown) {
    return request<T>('DELETE', path, body)
  },
  /** 以 multipart/form-data 上传单个文件；复用统一的 401 刷新逻辑。 */
  async upload<T>(path: string, file: File): Promise<T> {
    const res = await send(path, undefined, () => {
      const form = new FormData()
      form.append('file', file)
      const headers: Record<string, string> = {}
      if (tokenStore.access) headers.Authorization = `Bearer ${tokenStore.access}`
      return { method: 'POST', headers, body: form }
    })
    return parse<T>(res)
  },
  /** 下载二进制文件；错误响应仍按 JSON 包裹解析提示信息。 */
  async download(path: string, params?: Record<string, unknown>): Promise<Blob> {
    const res = await send(path, params, () => {
      const headers: Record<string, string> = {}
      if (tokenStore.access) headers.Authorization = `Bearer ${tokenStore.access}`
      return { headers }
    })
    if (!res.ok) {
      let message = '下载失败'
      try {
        const body = (await res.json()) as Envelope<unknown>
        message = body.message || message
      } catch {
        // 错误响应可能不是 JSON，解析失败时保留默认提示。
      }
      throw new ApiError(res.status, message)
    }
    return res.blob()
  },
}

/** 触发浏览器下载，并在点击后释放临时 Object URL。 */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
