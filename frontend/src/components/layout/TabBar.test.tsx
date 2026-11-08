/**
 * 页签右键菜单与关闭策略测试。
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useTabs, type Tab } from '../../stores/tabs'
import { TabBar } from './TabBar'

const dashboard: Tab = { key: '/dashboard', label: '总览', path: '/dashboard' }
const users: Tab = { key: '/system/users', label: '用户管理', path: '/system/users' }
const roles: Tab = { key: '/system/roles', label: '角色管理', path: '/system/roles' }

function renderTabs(props: { autoHidden?: boolean } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const ui = (autoHidden: boolean) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[users.path]}>
        <TabBar autoHidden={autoHidden} />
      </MemoryRouter>
    </QueryClientProvider>
  )
  const view = render(ui(props.autoHidden ?? false))
  return { ...view, rerenderWith: (autoHidden: boolean) => view.rerender(ui(autoHidden)) }
}

function chip(label: string) {
  const element = [...document.querySelectorAll('.tab-chip')].find((item) => item.textContent?.includes(label))
  if (!element) throw new Error(`tab chip not found: ${label}`)
  return element
}

// 在指定页签上触发右键，返回菜单容器。
function openMenu(label: string) {
  fireEvent.contextMenu(chip(label), { clientX: 320, clientY: 120 })
  return screen.getByRole('menu')
}

beforeEach(() => {
  useTabs.setState({ tabs: [dashboard, users, roles], active: users.key, refreshKey: 0 })
})

afterEach(() => {
  cleanup()
  useTabs.setState({ tabs: [], active: '', refreshKey: 0 })
})

describe('TabBar context menu', () => {
  it('opens on right click and exposes every action', () => {
    renderTabs()
    openMenu('用户管理')

    for (const name of ['刷新', '关闭', '关闭其他', '关闭全部']) {
      expect((screen.getByRole('menuitem', { name }) as HTMLButtonElement).disabled).toBe(false)
    }
  })

  it('disables close-left on the first tab and close-right on the last', () => {
    renderTabs()
    openMenu('总览')
    expect((screen.getByRole('menuitem', { name: '关闭左侧' }) as HTMLButtonElement).disabled).toBe(true)

    openMenu('角色管理')
    expect((screen.getByRole('menuitem', { name: '关闭右侧' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('closes other tabs from the action', () => {
    renderTabs()
    openMenu('用户管理')
    fireEvent.click(screen.getByRole('menuitem', { name: '关闭其他' }))

    expect(useTabs.getState().tabs).toEqual([users])
    expect(useTabs.getState().active).toBe(users.key)
  })

  it('closes tabs to the left of the target', () => {
    renderTabs()
    openMenu('用户管理')
    fireEvent.click(screen.getByRole('menuitem', { name: '关闭左侧' }))

    expect(useTabs.getState().tabs).toEqual([users, roles])
    expect(useTabs.getState().active).toBe(users.key)
  })

  it('refreshes the target and bumps the remount key', () => {
    renderTabs()
    openMenu('用户管理')
    fireEvent.click(screen.getByRole('menuitem', { name: '刷新' }))

    expect(useTabs.getState().active).toBe(users.key)
    expect(useTabs.getState().refreshKey).toBe(1)
    return waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  })

  it('closes on Escape and outside pointer down', async () => {
    renderTabs()
    openMenu('用户管理')
    fireEvent.keyDown(window, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())

    openMenu('用户管理')
    fireEvent.pointerDown(document.body)
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  })
})

describe('TabBar auto hide', () => {
  it('collapses when auto hidden and keeps chips mounted for a smooth restore', () => {
    const { rerenderWith } = renderTabs()
    const wrap = document.querySelector('.tab-strip-wrap') as HTMLElement
    expect(wrap.style.pointerEvents).toBe('auto')

    rerenderWith(true)
    expect(wrap.style.pointerEvents).toBe('none')
    // chip 保持挂载，回滚展开时不重放入场动画。
    expect(document.querySelectorAll('.tab-chip').length).toBe(3)

    rerenderWith(false)
    expect(wrap.style.pointerEvents).toBe('auto')
  })
})

describe('TabBar mobile switcher', () => {
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

  it('shows only the active tab and opens the full tab sheet', async () => {
    renderTabs()

    expect(document.querySelector('.mobile-tabbar')).toBeTruthy()
    expect(document.querySelector('.tab-strip')).toBeNull()
    expect(document.querySelector('.mobile-tab-label')?.textContent).toBe('用户管理')
    expect(document.querySelector('.mobile-tab-switch')?.textContent).toContain('3')

    fireEvent.click(screen.getByRole('button', { name: /切换页签/ }))
    await waitFor(() => expect(document.querySelectorAll('.tab-sheet-item').length).toBe(3))
    expect(screen.getByText('打开的页签 · 3')).toBeTruthy()
  })
})
