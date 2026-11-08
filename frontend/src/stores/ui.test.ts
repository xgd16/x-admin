/**
 * 界面偏好 store：主题色板、自动锁屏时长与持久化迁移。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DEFAULT_PALETTE } from '../config/themes'
import { useUi } from './ui'

function reset() {
  useUi.setState({ theme: 'light', palette: DEFAULT_PALETTE, density: 'middle', watermark: false, lockIdleMinutes: 10 })
}

describe('useUi palette', () => {
  beforeEach(reset)
  afterEach(reset)

  it('defaults to the coral palette', () => {
    expect(useUi.getState().palette).toBe(DEFAULT_PALETTE)
    expect(useUi.getState().palette).toBe('coral')
  })

  it('switches palette and persists the choice', () => {
    useUi.getState().setPalette('ocean')

    expect(useUi.getState().palette).toBe('ocean')
    const persisted = JSON.parse(localStorage.getItem('x-admin.ui') ?? '{}')
    expect(persisted.state.palette).toBe('ocean')
  })
})

describe('useUi lockIdleMinutes', () => {
  beforeEach(reset)
  afterEach(reset)

  it('defaults to 10 minutes and persists changes', () => {
    expect(useUi.getState().lockIdleMinutes).toBe(10)

    useUi.getState().setLockIdleMinutes(30)
    expect(useUi.getState().lockIdleMinutes).toBe(30)
    const persisted = JSON.parse(localStorage.getItem('x-admin.ui') ?? '{}')
    expect(persisted.state.lockIdleMinutes).toBe(30)

    useUi.getState().setLockIdleMinutes(0)
    expect(useUi.getState().lockIdleMinutes).toBe(0)
  })

  it('rejects values outside the option list', () => {
    useUi.getState().setLockIdleMinutes(7)
    expect(useUi.getState().lockIdleMinutes).toBe(10)
  })

  it('migrates v1 persisted state missing lockIdleMinutes', async () => {
    localStorage.setItem(
      'x-admin.ui',
      JSON.stringify({
        state: { theme: 'dark', density: 'small', watermark: true, columns: {} },
        version: 1,
      }),
    )

    // 重新加载模块会基于 localStorage 重建 store，触发 persist 的 migrate。
    vi.resetModules()
    const { useUi: reloaded } = (await import('./ui')) as typeof import('./ui')

    expect(reloaded.getState().lockIdleMinutes).toBe(10)
    expect(reloaded.getState().theme).toBe('dark')
    // migrate 产生的合法状态回写后版本号为 2。
    const persisted = JSON.parse(localStorage.getItem('x-admin.ui') ?? '{}')
    expect(persisted.version).toBe(2)
  })
})
