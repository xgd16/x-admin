/**
 * 锁屏 store：持久化、解锁调用、错误映射与多标签页联动。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { vi as viCore } from 'vitest'
import { tokenStore } from '../api/tokens'
import { bindLockAcrossTabs, useLock } from './lock'

// WebAuthn 依赖浏览器 API，jsdom 下统一打桩；解锁文案映射保持原实现。
vi.mock('../utils/passkey', () => ({
  isPasskeySupported: () => true,
  getPasskeyAssertion: viCore.fn(async () => ({ id: 'cred-x', type: 'public-key' })),
  passkeyErrorMessage: (error: unknown) => (error instanceof Error ? error.message : ''),
  base64urlToBuffer: viCore.fn(),
  bufferToBase64url: viCore.fn(),
}))

const ok = <T,>(data: T) => new Response(JSON.stringify({ code: 0, message: '', data }), { status: 200 })
const envelope = (code: number, message: string) =>
  new Response(JSON.stringify({ code, message, data: null }), { status: 200 })
const errOf = async <T>(p: Promise<T>): Promise<Error> =>
  (await p.catch((e: unknown) => e)) as Error

function reset() {
  useLock.setState({ locked: false, lockedAt: 0 })
  localStorage.removeItem('x-admin.lock')
}

describe('useLock', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    tokenStore.set('acc', 'ref')
    reset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    tokenStore.clear()
    reset()
  })

  it('lock 置位并持久化，reset 复位', () => {
    useLock.getState().lock()
    expect(useLock.getState().locked).toBe(true)
    expect(useLock.getState().lockedAt).toBeGreaterThan(0)
    const persisted = JSON.parse(localStorage.getItem('x-admin.lock') ?? '{}')
    expect(persisted.state.locked).toBe(true)

    useLock.getState().reset()
    expect(useLock.getState().locked).toBe(false)
    expect(JSON.parse(localStorage.getItem('x-admin.lock') ?? '{}').state.locked).toBe(false)
  })

  it('密码解锁成功：调用接口、携带令牌并复位', async () => {
    fetchMock.mockResolvedValue(ok(null))
    useLock.getState().lock()

    await useLock.getState().unlockWithPassword('Admin@123456')

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/admin/v1/auth/unlock')
    expect(JSON.parse(init.body)).toEqual({ password: 'Admin@123456' })
    expect(init.headers.Authorization).toBe('Bearer acc')
    expect(useLock.getState().locked).toBe(false)
  })

  it('密码错误：抛出后端消息且保持锁定', async () => {
    fetchMock.mockResolvedValue(envelope(40101, '密码错误'))
    useLock.getState().lock()

    const err = await errOf(useLock.getState().unlockWithPassword('bad'))

    expect(err.message).toBe('密码错误')
    expect(useLock.getState().locked).toBe(true)
  })

  it('通行密钥解锁：begin/assertion/finish 三段并复位', async () => {
    fetchMock
      .mockResolvedValueOnce(ok({ session_id: 's1', options: { publicKey: {} } }))
      .mockResolvedValueOnce(ok(null))
    useLock.getState().lock()

    await useLock.getState().unlockWithPasskey()

    expect(fetchMock.mock.calls[0][0]).toBe('/admin/v1/auth/unlock/passkey/begin')
    const [, finishInit] = fetchMock.mock.calls[1]
    expect(fetchMock.mock.calls[1][0]).toBe('/admin/v1/auth/unlock/passkey/finish')
    const body = JSON.parse(finishInit.body)
    expect(body.session_id).toBe('s1')
    expect(body.credential).toEqual({ id: 'cred-x', type: 'public-key' })
    expect(useLock.getState().locked).toBe(false)
  })

  it('通行密钥未绑定（40401）映射为友好文案', async () => {
    fetchMock.mockResolvedValue(envelope(40401, '还没有绑定通行密钥'))
    useLock.getState().lock()

    const err = await errOf(useLock.getState().unlockWithPasskey())

    expect(err.message).toBe('当前账号还没有绑定通行密钥')
    expect(useLock.getState().locked).toBe(true)
  })

  it('bindLockAcrossTabs 同步其他标签页的锁定与解锁', () => {
    const unbind = bindLockAcrossTabs()

    window.dispatchEvent(
      new StorageEvent('storage', {
        key: 'x-admin.lock',
        newValue: JSON.stringify({ state: { locked: true, lockedAt: 123 } }),
      }),
    )
    expect(useLock.getState().locked).toBe(true)
    expect(useLock.getState().lockedAt).toBe(123)

    window.dispatchEvent(
      new StorageEvent('storage', {
        key: 'x-admin.lock',
        newValue: JSON.stringify({ state: { locked: false, lockedAt: 0 } }),
      }),
    )
    expect(useLock.getState().locked).toBe(false)

    // 无关 key 与坏数据不影响本标签页。
    useLock.getState().lock()
    window.dispatchEvent(new StorageEvent('storage', { key: 'x-admin.ui', newValue: '{}' }))
    window.dispatchEvent(new StorageEvent('storage', { key: 'x-admin.lock', newValue: 'not-json' }))
    expect(useLock.getState().locked).toBe(true)

    unbind()
  })
})
