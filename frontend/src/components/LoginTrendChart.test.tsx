import { cleanup, fireEvent, render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoginTrendChart } from './LoginTrendChart'

const trend = [
  { date: '2026-10-28', count: 0 },
  { date: '2026-10-29', count: 0 },
  { date: '2026-10-30', count: 0 },
  { date: '2026-10-31', count: 0 },
  { date: '2026-11-01', count: 0 },
  { date: '2026-11-02', count: 0 },
  { date: '2026-11-03', count: 7 },
]

function mockChartRect(svg: SVGSVGElement) {
  vi.spyOn(svg, 'getBoundingClientRect').mockReturnValue({
    x: 0,
    y: 0,
    top: 0,
    left: 0,
    right: 760,
    bottom: 246,
    width: 760,
    height: 246,
    toJSON: () => ({}),
  } as DOMRect)
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('LoginTrendChart', () => {
  it('stays clean until the pointer enters the plot', () => {
    const { container } = render(<LoginTrendChart trend={trend} />)

    expect(container.querySelector('.chart-tooltip')).toBeNull()
    expect(container.querySelectorAll('.chart circle')).toHaveLength(trend.length)
  })

  it('follows the pointer and hides on leave', () => {
    const { container } = render(<LoginTrendChart trend={trend} />)
    const svg = container.querySelector('svg') as SVGSVGElement
    const hit = container.querySelector('.chart-hit') as SVGRectElement
    mockChartRect(svg)

    fireEvent.mouseMove(hit, { clientX: 42, clientY: 120 })
    let tooltip = container.querySelector('.chart-tooltip')
    expect(tooltip?.textContent).toContain('0 次')
    expect(tooltip?.textContent).toContain('10-28')
    expect(tooltip?.textContent).toContain('当日')
    expect(container.querySelectorAll('.chart circle')).toHaveLength(trend.length + 2)

    fireEvent.mouseMove(hit, { clientX: 742, clientY: 120 })
    tooltip = container.querySelector('.chart-tooltip')
    expect(tooltip?.textContent).toContain('7 次')
    expect(tooltip?.textContent).toContain('峰值')

    fireEvent.mouseLeave(hit)
    expect(container.querySelector('.chart-tooltip')).toBeNull()
    expect(container.querySelectorAll('.chart circle')).toHaveLength(trend.length)
  })

  it('has no tooltip or hit area without data', () => {
    const { container } = render(<LoginTrendChart trend={[]} />)

    expect(container.querySelector('.chart-tooltip')).toBeNull()
    expect(container.querySelector('.chart-hit')).toBeNull()
  })
})
