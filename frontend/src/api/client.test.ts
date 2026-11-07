/**
 * 统一 HTTP 客户端测试：令牌注入、错误解析、401 单飞刷新、下载与上传。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api, saveBlob } from './client'
import { tokenStore } from './tokens'

// 统一响应包裹。
const ok = <T,>(data: T) =>
  new Response(JSON.stringify({ code: 0, message: '', data }), { status: 200 })

const envelope = (code: number, message: string) =>
  new Response(JSON.stringify({ code, message, data: null }), { status: 200 })

// 捕获被拒绝的 Promise 并按 ApiError 断言，避免 success 类型与错误的联合类型。
const errOf = async <T>(p: Promise<T>): Promise<ApiError> =>
  (await p.catch((e: unknown) => e)) as ApiError

describe('api client', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    tokenStore.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    tokenStore.clear()
  })

  it('成功请求返回 data 并注入 Bearer 头', async () => {
    tokenStore.set('acc', 'ref')
    fetchMock.mockResolvedValue(ok({ id: 7 }))

    await expect(api.get('/users/7')).resolves.toEqual({ id: 7 })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/admin/v1/users/7')
    expect(init.headers.Authorization).toBe('Bearer acc')
    expect(init.headers['Content-Type']).toBeUndefined()
  })

  it('业务 code 非 0 抛出 ApiError', async () => {
    fetchMock.mockResolvedValue(envelope(40001, '参数错误'))
    const err = await errOf(api.get('/x'))
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe(40001)
    expect(err.message).toBe('参数错误')
  })

  it('HTTP 401 且 code 缺失时使用状态码', async () => {
    fetchMock.mockResolvedValue(new Response('{"message":"denied"}', { status: 401 }))
    const err = await errOf(api.get('/x'))
    expect(err.code).toBe(401)
  })

  it('查询参数忽略 undefined / null / 空串', async () => {
    fetchMock.mockResolvedValue(ok(null))
    await api.get('/users', { page: 1, q: '', empty: undefined, nil: null, name: '李四' })
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toBe('/admin/v1/users?page=1&name=' + encodeURIComponent('李四'))
  })

  it('401 时自动刷新令牌并重放原请求', async () => {
    tokenStore.set('stale', 'refresh-me')
    const unauthorized = new Response('{"code":40102,"message":"expired"}', { status: 401 })
    fetchMock
      .mockResolvedValueOnce(unauthorized) // 原请求 401
      .mockResolvedValueOnce(ok({ access_token: 'new-acc', refresh_token: 'new-ref' })) // 刷新
      .mockResolvedValueOnce(ok({ id: 1 })) // 重放成功

    await expect(api.get('/me')).resolves.toEqual({ id: 1 })
    expect(fetchMock).toHaveBeenCalledTimes(3)
    const [refreshUrl, refreshInit] = fetchMock.mock.calls[1]
    expect(refreshUrl).toBe('/admin/v1/auth/refresh')
    expect(JSON.parse(refreshInit.body)).toEqual({ refresh_token: 'refresh-me' })
    expect(tokenStore.access).toBe('new-acc')
    // 重放请求携带新令牌。
    expect(fetchMock.mock.calls[2][1].headers.Authorization).toBe('Bearer new-acc')
  })

  it('认证接口自身 401 不触发刷新', async () => {
    tokenStore.set('acc', 'ref')
    fetchMock.mockResolvedValue(new Response('{"code":40102}', { status: 401 }))
    await expect(api.post('/auth/login', {})).rejects.toBeInstanceOf(ApiError)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('刷新失败清空令牌并跳转登录页', async () => {
    tokenStore.set('acc', 'bad-refresh')
    fetchMock
      .mockResolvedValueOnce(new Response('{"code":40102}', { status: 401 }))
      .mockResolvedValueOnce(new Response('{"code":40102}', { status: 401 }))
    const assign = vi.fn()
    Object.defineProperty(window, 'location', { value: { assign }, writable: true })

    await expect(api.get('/me')).rejects.toMatchObject({ code: 40102 })
    expect(assign).toHaveBeenCalledWith('/login')
    expect(tokenStore.access).toBe('')
    expect(tokenStore.refresh).toBe('')
  })

  it('并发 401 只发起一次刷新', async () => {
    tokenStore.set('stale', 'ref')
    const unauthorized = () => new Response('{"code":40102}', { status: 401 })
    fetchMock
      .mockResolvedValueOnce(unauthorized()) // 请求 A 401
      .mockResolvedValueOnce(unauthorized()) // 请求 B 401
      .mockResolvedValueOnce(ok({ access_token: 'a2', refresh_token: 'r2' })) // 单飞刷新
      .mockResolvedValueOnce(ok({ id: 1 })) // A 重放
      .mockResolvedValueOnce(ok({ id: 2 })) // B 重放

    const [first, second] = await Promise.all([api.get('/me'), api.get('/me')])
    expect(first).toEqual({ id: 1 })
    expect(second).toEqual({ id: 2 })
    // 2 个原请求 + 1 次刷新 + 2 次重放。
    expect(fetchMock).toHaveBeenCalledTimes(5)
  })

  it('upload 以 multipart 发送且不带 JSON Content-Type', async () => {
    tokenStore.set('acc', 'ref')
    fetchMock.mockResolvedValue(ok({ id: 9 }))
    const file = new File(['hello'], 'a.png', { type: 'image/png' })
    await expect(api.upload('/files', file)).resolves.toEqual({ id: 9 })
    const [, init] = fetchMock.mock.calls[0]
    expect(init.body).toBeInstanceOf(FormData)
    expect(init.headers['Content-Type']).toBeUndefined()
    expect(init.headers.Authorization).toBe('Bearer acc')
  })

  it('download 成功返回 Blob', async () => {
    tokenStore.set('acc', 'ref')
    fetchMock.mockResolvedValue(new Response('binary-content', { status: 200 }))
    const blob = await api.download('/users/export')
    expect(await blob.text()).toBe('binary-content')
    expect(fetchMock.mock.calls[0][0]).toBe('/admin/v1/users/export')
  })

  it('download 错误响应解析 JSON 提示', async () => {
    fetchMock.mockResolvedValue(new Response('{"code":40301,"message":"没有权限"}', { status: 403 }))
    const err = await errOf(api.download('/users/export'))
    expect(err.code).toBe(403)
    expect(err.message).toBe('没有权限')
  })

  it('download 错误响应非 JSON 时保留默认提示', async () => {
    fetchMock.mockResolvedValue(new Response('<html>oops</html>', { status: 502 }))
    const err = await errOf(api.download('/users/export'))
    expect(err.code).toBe(502)
    expect(err.message).toBe('下载失败')
  })

  it('saveBlob 触发下载并释放 ObjectURL', () => {
    const createObjectURL = vi.fn(() => 'blob:mock')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', { value: createObjectURL, configurable: true })
    Object.defineProperty(URL, 'revokeObjectURL', { value: revokeObjectURL, configurable: true })

    saveBlob(new Blob(['x']), '报表.xlsx')
    expect(createObjectURL).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:mock')
  })
})

describe('tokenStore', () => {
  it('set 持久化到 localStorage，clear 清空内存与存储', () => {
    tokenStore.set('a', 'b')
    expect(localStorage.getItem('x-admin.access')).toBe('a')
    expect(localStorage.getItem('x-admin.refresh')).toBe('b')
    expect(tokenStore.access).toBe('a')

    tokenStore.clear()
    expect(localStorage.getItem('x-admin.access')).toBeNull()
    expect(localStorage.getItem('x-admin.refresh')).toBeNull()
    expect(tokenStore.access).toBe('')
    expect(tokenStore.refresh).toBe('')
  })
})
