import { act, cleanup, render } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const chartMock = vi.hoisted(() => {
  const chart = {
    setOption: vi.fn((_option: unknown) => undefined),
    resize: vi.fn(),
    dispose: vi.fn(),
  }
  return {
    chart,
    init: vi.fn((_host: HTMLElement, _theme?: string, _opts?: unknown) => chart),
  }
})

vi.mock('../charts/echarts', () => ({ echarts: { init: chartMock.init } }))

import { LoginTrendChart } from './LoginTrendChart'
import { useUi } from '../stores/ui'

const trend = [
  { date: '2026-10-28', count: 0 },
  { date: '2026-10-29', count: 2 },
  { date: '2026-10-30', count: 1 },
  { date: '2026-10-31', count: 3 },
  { date: '2026-11-01', count: 0 },
  { date: '2026-11-02', count: 5 },
  { date: '2026-11-03', count: 7 },
]

interface OptionShape {
  series: Array<{ data: number[] }>
  yAxis: { splitLine: { lineStyle: { color: string } } }
}

function lastOption(): OptionShape {
  const calls = chartMock.chart.setOption.mock.calls
  return calls[calls.length - 1][0] as OptionShape
}

beforeEach(() => {
  chartMock.init.mockClear()
  chartMock.chart.setOption.mockClear()
  chartMock.chart.resize.mockClear()
  chartMock.chart.dispose.mockClear()
  useUi.setState({ theme: 'light' })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('LoginTrendChart', () => {
  it('mounts one ECharts instance and feeds it the daily counts', () => {
    const { container } = render(<LoginTrendChart trend={trend} />)
    const host = container.querySelector('.chart')

    expect(host).not.toBeNull()
    expect(chartMock.init).toHaveBeenCalledTimes(1)
    expect(chartMock.init.mock.calls[0][0]).toBe(host)
    expect(lastOption().series[0].data).toEqual(trend.map((point) => point.count))
  })

  it('repaints the palette when the theme changes', () => {
    render(<LoginTrendChart trend={trend} />)

    act(() => {
      useUi.setState({ theme: 'dark' })
    })

    expect(lastOption().yAxis.splitLine.lineStyle.color).toBe('rgba(176, 202, 230, 0.13)')
    expect(chartMock.chart.setOption.mock.calls.length).toBeGreaterThan(1)
  })

  it('shows an empty state without data', () => {
    const { container } = render(<LoginTrendChart trend={[]} />)

    expect(container.querySelector('.chart-empty')?.textContent).toBe('暂无数据')
    expect(lastOption().series[0].data).toEqual([])
  })

  it('disposes the instance on unmount', () => {
    const { unmount } = render(<LoginTrendChart trend={trend} />)

    unmount()

    expect(chartMock.chart.dispose).toHaveBeenCalledTimes(1)
  })
})
