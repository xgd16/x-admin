/**
 * 闲置自动锁屏：计时、合成事件防续命与关闭档位。
 */
import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useLock } from '../stores/lock'
import { useUi } from '../stores/ui'
import { useIdleLock } from './useIdleLock'

describe('useIdleLock', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    useUi.setState({ lockIdleMinutes: 5 })
    useLock.setState({ locked: false, lockedAt: 0 })
  })

  afterEach(() => {
    vi.useRealTimers()
    useUi.setState({ lockIdleMinutes: 10 })
    useLock.setState({ locked: false, lockedAt: 0 })
  })

  it('闲置达到设定时长后自动锁定', () => {
    renderHook(() => useIdleLock())

    vi.advanceTimersByTime(5 * 60_000 - 1)
    expect(useLock.getState().locked).toBe(false)
    vi.advanceTimersByTime(1)
    expect(useLock.getState().locked).toBe(true)
  })

  it('脚本合成的事件不重置计时，避免程序化滚动续命', () => {
    renderHook(() => useIdleLock())

    vi.advanceTimersByTime(4 * 60_000)
    window.dispatchEvent(new Event('pointermove'))
    window.dispatchEvent(new Event('keydown'))
    window.dispatchEvent(new Event('wheel'))
    vi.advanceTimersByTime(1 * 60_000)

    expect(useLock.getState().locked).toBe(true)
  })

  it('时长为 0 时不启用自动锁', () => {
    useUi.setState({ lockIdleMinutes: 0 })
    renderHook(() => useIdleLock())

    vi.advanceTimersByTime(60 * 60_000)

    expect(useLock.getState().locked).toBe(false)
  })

  it('已锁定时挂起计时，解锁后重新武装', () => {
    useLock.setState({ locked: true, lockedAt: 1 })
    const { rerender } = renderHook(() => useIdleLock())

    vi.advanceTimersByTime(30 * 60_000)
    expect(useLock.getState().locked).toBe(true)

    useLock.setState({ locked: false, lockedAt: 0 })
    rerender()
    vi.advanceTimersByTime(5 * 60_000)
    expect(useLock.getState().locked).toBe(true)
  })

  it('调整时长后按新时长计时', () => {
    const { rerender } = renderHook(() => useIdleLock())
    useUi.setState({ lockIdleMinutes: 1 })
    rerender()

    vi.advanceTimersByTime(59_999)
    expect(useLock.getState().locked).toBe(false)
    vi.advanceTimersByTime(1)
    expect(useLock.getState().locked).toBe(true)
  })
})
