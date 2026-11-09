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

function scrolling() {
  return document.documentElement.dataset.canvasScrolling
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
  delete document.documentElement.dataset.canvasScrolled
  delete document.documentElement.dataset.canvasScrolling
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

  it('marks active scrolling and clears it after the idle window', () => {
    const { scrollTo } = mount()
    scrollTo(240)
    expect(scrolling()).toBe('on')
    act(() => {
      vi.advanceTimersByTime(139)
    })
    expect(scrolling()).toBe('on')
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(scrolling()).toBe('off')
  })

  it('restarts the idle window on every scroll event', () => {
    const { scrollTo } = mount()
    scrollTo(240)
    act(() => {
      vi.advanceTimersByTime(120)
    })
    scrollTo(300)
    act(() => {
      vi.advanceTimersByTime(120)
    })
    expect(scrolling()).toBe('on')
  })

  it('clears the state when scrolled back to the top', () => {
    const { scrollTo } = mount()
    scrollTo(240)
    scrollTo(0)
    expect(scrolled()).toBe('off')
  })

  it('clears both root attributes when unmounted', () => {
    const { hook, scrollTo } = mount()
    scrollTo(240)
    hook.unmount()
    expect(scrolled()).toBeUndefined()
    expect(scrolling()).toBeUndefined()
  })
})
