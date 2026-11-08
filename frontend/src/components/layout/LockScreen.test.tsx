/**
 * 锁屏覆盖层：锁定渲染、密码解锁与退出逃生口。
 */
import { App as AntdApp } from 'antd'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenStore } from '../../api/tokens'
import { useAuth } from '../../stores/auth'
import { useLock } from '../../stores/lock'
import { LockScreen } from './LockScreen'

// jsdom 没有 WebAuthn；锁屏测试固定走密码路径，隐藏通行密钥按钮。
vi.mock('../../utils/passkey', () => ({
  isPasskeySupported: () => false,
  getPasskeyAssertion: vi.fn(),
  passkeyErrorMessage: (error: unknown) => (error instanceof Error ? error.message : ''),
}))

const ok = <T,>(data: T) => new Response(JSON.stringify({ code: 0, message: '', data }), { status: 200 })
const envelope = (code: number, message: string) =>
  new Response(JSON.stringify({ code, message, data: null }), { status: 200 })

function renderLock() {
  return render(
    <AntdApp>
      <MemoryRouter>
        <LockScreen />
      </MemoryRouter>
    </AntdApp>,
  )
}

const mockUser = {
  id: 1, username: 'admin', nickname: '管理员', email: '', phone: '', avatar: '',
  dept_id: null, status: 1 as const, last_login_at: null, last_login_ip: '',
  remark: '', totp_enabled: false, created_at: '',
}

describe('LockScreen', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    tokenStore.set('acc', 'ref')
    useAuth.setState({ user: mockUser, status: 'ready' })
    useLock.setState({ locked: false, lockedAt: 0 })
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    tokenStore.clear()
    useLock.setState({ locked: false, lockedAt: 0 })
  })

  it('未锁定时不渲染覆盖层', () => {
    renderLock()
    expect(document.querySelector('.lock-screen')).toBeNull()
    expect(document.querySelector('.lock-card')).toBeNull()
  })

  it('锁定时展示用户与时钟，密码正确后覆盖层消失', async () => {
    useLock.setState({ locked: true, lockedAt: Date.now() })
    fetchMock.mockResolvedValue(ok(null))
    renderLock()

    expect(document.querySelector('.lock-screen')).toBeTruthy()
    expect(screen.getByText('管理员')).toBeTruthy()
    expect(screen.getByText('屏幕已锁定，请重新验证身份')).toBeTruthy()
    expect(document.querySelector('.lock-time')?.textContent).toMatch(/^\d{2}:\d{2}$/)

    fireEvent.change(screen.getByPlaceholderText('输入密码解锁'), { target: { value: 'Admin@123456' } })
    fireEvent.click(screen.getByRole('button', { name: /解\s*锁/ }))

    await waitFor(() => expect(useLock.getState().locked).toBe(false))
    await waitFor(() => expect(document.querySelector('.lock-screen')).toBeNull())
  })

  it('密码错误显示告警并保持锁定', async () => {
    useLock.setState({ locked: true, lockedAt: Date.now() })
    fetchMock.mockResolvedValue(envelope(40101, '密码错误'))
    renderLock()

    fireEvent.change(screen.getByPlaceholderText('输入密码解锁'), { target: { value: 'bad' } })
    fireEvent.click(screen.getByRole('button', { name: /解\s*锁/ }))

    await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy())
    expect(screen.getByRole('alert').textContent).toBe('密码错误')
    expect(useLock.getState().locked).toBe(true)
    expect(document.querySelector('.lock-screen')).toBeTruthy()
  })

  it('通行密钥不可用时隐藏对应按钮', () => {
    useLock.setState({ locked: true, lockedAt: Date.now() })
    renderLock()
    expect(screen.queryByRole('button', { name: /通行密钥/ })).toBeNull()
  })

  it('退出登录逃生口清锁并跳转登录页', async () => {
    useLock.setState({ locked: true, lockedAt: Date.now() })
    fetchMock.mockResolvedValue(ok(null))
    renderLock()

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }))

    await waitFor(() => expect(useLock.getState().locked).toBe(false))
    const urls = fetchMock.mock.calls.map(([url]) => String(url))
    expect(urls.some((url) => url.includes('/auth/logout'))).toBe(true)
    expect(useAuth.getState().status).toBe('anon')
  })
})
