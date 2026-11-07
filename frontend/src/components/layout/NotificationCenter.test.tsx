/**
 * 通知中心未读数、已读与空状态测试。
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { App as AntdApp } from 'antd'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../api/client'
import type { Notification, PageResult } from '../../api/types'
import { NotificationCenter } from './NotificationCenter'

vi.mock('../../api/client', () => ({
  api: { get: vi.fn(), put: vi.fn() },
}))

const rows: Notification[] = [
  {
    id: 7,
    user_id: 1,
    title: '新登录提醒',
    content: '你的账号于 11-03 18:20 在 127.0.0.1（macOS）登录。',
    type: 'info',
    link: '/profile',
    actor_id: 0,
    read_at: null,
    created_at: '2026-11-03T18:20:00+08:00',
  },
  {
    id: 6,
    user_id: 1,
    title: '密码修改成功',
    content: '你的登录密码已更新。',
    type: 'success',
    link: '/profile',
    actor_id: 0,
    read_at: '2026-11-03T18:00:00+08:00',
    created_at: '2026-11-03T17:59:00+08:00',
  },
]

const page: PageResult<Notification> = { list: rows, total: rows.length, page: 1, page_size: 20 }

// 默认返回 1 条未读；不同用例可按需覆盖空数据。
beforeEach(() => {
  vi.mocked(api.get).mockImplementation((path: string) => {
    if (path === '/notifications/unread-count') return Promise.resolve({ count: 1 })
    return Promise.resolve(page)
  })
  vi.mocked(api.put).mockResolvedValue(null)
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function renderCenter() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <AntdApp>
        <MemoryRouter>
          <NotificationCenter />
        </MemoryRouter>
      </AntdApp>
    </QueryClientProvider>,
  )
}

describe('NotificationCenter', () => {
  it('shows the real unread count and lists messages', async () => {
    renderCenter()

    const bell = await screen.findByRole('button', { name: '通知，1 条未读' })
    fireEvent.click(bell)

    expect(await screen.findByText('新登录提醒')).toBeTruthy()
    expect(screen.getByText('密码修改成功')).toBeTruthy()
    expect(screen.getByText('1 条未读')).toBeTruthy()
  })

  it('marks one message or every message as read', async () => {
    renderCenter()

    fireEvent.click(await screen.findByRole('button', { name: '通知，1 条未读' }))
    fireEvent.click(await screen.findByRole('button', { name: '全部已读' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledWith('/notifications/read-all'))

    fireEvent.click(screen.getByText('新登录提醒').closest('button') as HTMLButtonElement)
    await waitFor(() => expect(api.put).toHaveBeenCalledWith('/notifications/7/read'))
  })

  it('renders an empty state without a badge', async () => {
    vi.mocked(api.get).mockImplementation((path: string) => {
      if (path === '/notifications/unread-count') return Promise.resolve({ count: 0 })
      return Promise.resolve({ list: [], total: 0, page: 1, page_size: 20 })
    })
    renderCenter()

    fireEvent.click(await screen.findByRole('button', { name: '通知' }))
    expect(await screen.findByText('暂无通知')).toBeTruthy()
    expect(document.querySelector('.icon-btn .badge')).toBeNull()
  })
})

describe('NotificationCenter mobile sheet', () => {
  const originalMatchMedia = window.matchMedia

  beforeEach(() => {
    window.matchMedia = ((query: string) => ({
      matches: query.includes('max-width: 900px'),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as typeof window.matchMedia
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
  })

  it('opens notifications in a bottom sheet instead of an anchored popover', async () => {
    renderCenter()

    fireEvent.click(await screen.findByRole('button', { name: '通知，1 条未读' }))

    expect(await screen.findByText('新登录提醒')).toBeTruthy()
    expect(document.querySelector('.notif-sheet')).toBeTruthy()
    expect(document.querySelector('.notif-sheet-handle')).toBeTruthy()
    expect(document.querySelector('.notif-popover')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: '关闭通知' }))
    await waitFor(() => expect(document.querySelector('.notif-sheet')).toBeNull())
  })
})
