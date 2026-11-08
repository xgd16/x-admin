/**
 * 滚动方向收起判断测试。
 */
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { useScrollAutoHide } from './useScrollAutoHide'

function mount() {
  const el = document.createElement('div')
  document.body.appendChild(el)
  const { result } = renderHook(() => useScrollAutoHide({ current: el }))
  const scrollTo = (top: number) => {
    act(() => {
      el.scrollTop = top
      el.dispatchEvent(new Event('scroll'))
    })
  }
  const markProgrammatic = () => act(() => result.current[1]())
  return { el, result, scrollTo, markProgrammatic }
}

function cleanup() {
  document.body.innerHTML = ''
}

afterEach(cleanup)

describe('useScrollAutoHide', () => {
  it('keeps visible near the top regardless of direction', () => {
    const { scrollTo, result } = mount()
    scrollTo(30)
    expect(result.current[0]).toBe(false)
  })

  it('hides after a deliberate downward scroll and shows again on scroll up', () => {
    const { scrollTo, result } = mount()
    scrollTo(120)
    expect(result.current[0]).toBe(true)

    scrollTo(90)
    expect(result.current[0]).toBe(false)
  })

  it('accumulates small scroll steps before hiding', () => {
    const { scrollTo, result } = mount()
    // 先越过近顶区，再以小于阈值的步进缓慢下滚。
    scrollTo(30)
    scrollTo(48)
    expect(result.current[0]).toBe(false)
    scrollTo(60)
    expect(result.current[0]).toBe(true)
  })

  it('stays visible for programmatic scroll restores', () => {
    const { scrollTo, markProgrammatic, result } = mount()
    markProgrammatic()
    scrollTo(400)
    expect(result.current[0]).toBe(false)
  })

  it('expires the programmatic window when no scroll event fires', async () => {
    const { scrollTo, markProgrammatic, result } = mount()
    markProgrammatic()
    // 目标位置与当前相同 → 不产生事件；窗口过期后真实滚动照常收起。
    await new Promise((resolve) => setTimeout(resolve, 140))
    scrollTo(100)
    expect(result.current[0]).toBe(true)
  })
})
