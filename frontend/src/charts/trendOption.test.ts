import { describe, expect, it } from 'vitest'
import type { LinearGradientObject } from 'echarts/core'
import type { TrendChartOption } from './trendOption'
import { buildTrendOption, trendPeakIndex, trendTooltipHtml } from './trendOption'
import { chartPalette } from './theme'

const trend = [
  { date: '2026-10-28', count: 0 },
  { date: '2026-10-29', count: 2 },
  { date: '2026-10-30', count: 1 },
  { date: '2026-10-31', count: 3 },
  { date: '2026-11-01', count: 0 },
  { date: '2026-11-02', count: 4 },
  { date: '2026-11-03', count: 7 },
]

interface SeriesShape {
  data: number[]
  lineStyle?: { color?: unknown; shadowBlur?: number }
  itemStyle?: { color?: string; borderColor?: string }
  areaStyle?: { color?: unknown }
}

function seriesOf(option: TrendChartOption): SeriesShape {
  const series = option.series as unknown as SeriesShape[]
  return series[0]
}

function gradientOf(color: unknown): LinearGradientObject {
  return color as LinearGradientObject
}

function build(animate = false) {
  return buildTrendOption(trend, { palette: chartPalette('light'), animate })
}

describe('buildTrendOption', () => {
  it('maps the daily counts and dates onto the axes', () => {
    const option = build()
    const xAxis = option.xAxis as unknown as { data: string[] }

    expect(xAxis.data).toEqual(trend.map((point) => point.date))
    expect(seriesOf(option).data).toEqual(trend.map((point) => point.count))
  })

  it('keeps the coral gradient stroke, glow and fading area fill', () => {
    const series = seriesOf(build())
    const stroke = gradientOf(series.lineStyle?.color)
    const area = gradientOf(series.areaStyle?.color)

    expect(stroke.type).toBe('linear')
    expect(stroke.x2).toBe(1)
    expect(stroke.colorStops.map((stop) => stop.color)).toEqual([
      '#ff9a6c',
      '#ff5f33',
      '#ff4d8d',
    ])
    expect(series.lineStyle?.shadowBlur).toBe(12)

    expect(area.type).toBe('linear')
    expect(area.y2).toBe(1)
    expect(area.colorStops).toEqual([
      { offset: 0, color: 'rgba(255, 106, 61, 0.22)' },
      { offset: 1, color: 'rgba(255, 106, 61, 0)' },
    ])
  })

  it('repaints grid, dots and tooltip with the dark palette', () => {
    const option = buildTrendOption(trend, { palette: chartPalette('dark'), animate: false })
    const yAxis = option.yAxis as unknown as { splitLine: { lineStyle: { color: string } } }

    expect(yAxis.splitLine.lineStyle.color).toBe('rgba(176, 202, 230, 0.13)')
    expect(seriesOf(option).itemStyle?.color).toBe('#101827')
    expect(option.tooltip).toMatchObject({ backgroundColor: 'rgba(19, 27, 41, 0.94)' })
  })

  it('respects the reduced-motion flag', () => {
    expect(buildTrendOption(trend, { palette: chartPalette('light'), animate: false }).animation).toBe(
      false,
    )
    expect(buildTrendOption(trend, { palette: chartPalette('light') }).animation).toBe(true)
  })
})

describe('trendTooltipHtml', () => {
  it('marks the peak day and formats the count', () => {
    expect(trendTooltipHtml(trend, 6)).toBe('<b>7 次</b><span>11-03 · 峰值</span>')
    expect(trendTooltipHtml(trend, 5)).toBe('<b>4 次</b><span>11-02 · 当日</span>')
  })

  it('treats an all-zero series as ordinary days', () => {
    expect(trendTooltipHtml([{ date: '2026-11-03', count: 0 }], 0)).toContain('当日')
  })

  it('renders nothing for a missing index', () => {
    expect(trendTooltipHtml([], 0)).toBe('')
  })
})

describe('trendPeakIndex', () => {
  it('finds the busiest day and falls back to the first point', () => {
    expect(trendPeakIndex(trend)).toBe(6)
    expect(trendPeakIndex([{ date: '2026-11-03', count: 0 }])).toBe(0)
  })
})
