import { mockRequest } from '../mock'
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

export function isMock(): boolean {
  return localStorage.getItem('x-admin.mock') === '1'
}

let refreshing: Promise<boolean> | null = null

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

async function parse<T>(res: Response): Promise<T> {
  const body = (await res.json()) as Envelope<T>
  if (!res.ok || body.code !== 0) {
    throw new ApiError(body.code || res.status, body.message || '请求失败')
  }
  return body.data
}

async function request<T>(method: string, path: string, body?: unknown, params?: Record<string, unknown>): Promise<T> {
  if (isMock()) {
    return mockRequest<T>(method, path, params, body)
  }
  const url = `${BASE}${path}${queryString(params)}`
  const headers: Record<string, string> = {}
  if (body !== undefined && !(body instanceof FormData)) headers['Content-Type'] = 'application/json'
  if (tokenStore.access) headers.Authorization = `Bearer ${tokenStore.access}`

  let res = await fetch(url, {
    method,
    headers,
    body: body instanceof FormData ? body : body === undefined ? undefined : JSON.stringify(body),
  })

  if (res.status === 401 && tokenStore.refresh && path !== '/auth/login' && path !== '/auth/refresh') {
    refreshing ??= refreshToken().finally(() => { refreshing = null })
    const ok = await refreshing
    if (ok) {
      headers.Authorization = `Bearer ${tokenStore.access}`
      res = await fetch(url, {
        method,
        headers,
        body: body instanceof FormData ? body : body === undefined ? undefined : JSON.stringify(body),
      })
    } else {
      tokenStore.clear()
      window.location.assign('/login')
      throw new ApiError(40102, '登录已过期')
    }
  }
  return parse<T>(res)
}

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
  async upload<T>(path: string, file: File): Promise<T> {
    if (isMock()) return mockRequest<T>('POST', path, undefined, file)
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(`${BASE}${path}`, {
      method: 'POST',
      headers: tokenStore.access ? { Authorization: `Bearer ${tokenStore.access}` } : {},
      body: form,
    })
    return parse<T>(res)
  },
  async download(path: string, params?: Record<string, unknown>): Promise<Blob> {
    if (isMock()) return new Blob(['mock export'], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    const res = await fetch(`${BASE}${path}${queryString(params)}`, {
      headers: tokenStore.access ? { Authorization: `Bearer ${tokenStore.access}` } : {},
    })
    if (!res.ok) {
      let message = '下载失败'
      try {
        const body = (await res.json()) as Envelope<unknown>
        message = body.message || message
      } catch {
        // ignore non-json error bodies
      }
      throw new ApiError(res.status, message)
    }
    return res.blob()
  },
}

export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
