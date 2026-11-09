/**
 * 内容区滚动表面状态测试。
 */
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useCanvasScrollSurface } from './useCanvasScrollSurface'

function mount() {
  const element = document.createElement('div')
  document.body.appendChild(element)
  const hook = renderHook(() => useCanvasScrollSurface({ current: element }))
  const scrollTo = (top: number) => {
    act(() => {
      element.scrollTop = top
      element.dispatchEvent(new Event('scroll'))
    })
  }
  return { element, hook, scrollTo }
}

function scrolled() {
  return document.documentElement.dataset.canvasScrolled
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
  delete document.documentElement.dataset.canvasScrolled
})

describe('useCanvasScrollSurface', () => {
  it('marks the surface as scrolled while content is below the header', () => {
    const { scrollTo } = mount()
    scrollTo(240)
    expect(scrolled()).toBe('on')
  })

  it('keeps the frosted state after scrolling stops', () => {
    const { scrollTo } = mount()
    scrollTo(240)
    act(() => {
      vi.advanceTimersByTime(200)
    })
    expect(scrolled()).toBe('on')
  })

  it('clears the state when scrolled back to the top', () => {
    const { scrollTo } = mount()
    scrollTo(240)
    scrollTo(0)
    expect(scrolled()).toBe('off')
  })

  it('clears the root attribute when unmounted', () => {
    const { hook, scrollTo } = mount()
    scrollTo(240)
    hook.unmount()
    expect(scrolled()).toBeUndefined()
  })
})
