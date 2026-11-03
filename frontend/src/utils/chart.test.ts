import { describe, expect, it } from 'vitest'
import {
  CHART,
  CHART_PLOT_WIDTH,
  nearestTrendIndex,
  smoothPath,
  tooltipAnchor,
  trendPoints,
} from './chart'

describe('trendPoints', () => {
  it('spreads the series across the plot and scales the peak to the top', () => {
    const points = trendPoints([
      { date: '2026-10-28', count: 0 },
      { date: '2026-10-29', count: 5 },
      { date: '2026-10-30', count: 10 },
    ])

    expect(points).toHaveLength(3)
    expect(points[0]).toEqual({ x: CHART.left, y: CHART.baseline })
    expect(points[1].x).toBeCloseTo(CHART.left + CHART_PLOT_WIDTH / 2)
    expect(points[1].y).toBeCloseTo(CHART.baseline - CHART.plotHeight / 2)
    expect(points[2]).toEqual({ x: CHART.right, y: CHART.baseline - CHART.plotHeight })
  })

  it('keeps an all-zero series on the baseline', () => {
    const points = trendPoints([
      { date: '2026-10-28', count: 0 },
      { date: '2026-10-29', count: 0 },
    ])

    expect(points.map((point) => point.y)).toEqual([CHART.baseline, CHART.baseline])
  })

  it('centers a single point and handles an empty series', () => {
    expect(trendPoints([{ date: '2026-10-28', count: 4 }])).toEqual([
      { x: CHART.left, y: CHART.baseline - CHART.plotHeight },
    ])
    expect(trendPoints([])).toEqual([])
  })
})

describe('smoothPath', () => {
  it('builds a move command plus one cubic segment per gap', () => {
    const path = smoothPath([
      { x: 0, y: 10 },
      { x: 10, y: 20 },
      { x: 20, y: 30 },
    ])

    expect(path).toBe('M0 10 C 5 10, 5 20, 10 20 C 15 20, 15 30, 20 30')
    expect(smoothPath([{ x: 3, y: 4 }])).toBe('M3 4')
    expect(smoothPath([])).toBe('')
  })
})

describe('nearestTrendIndex', () => {
  const rect = { left: 0, width: CHART.width }

  it('snaps to the nearest point and clamps outside the plot', () => {
    expect(nearestTrendIndex(CHART.left, rect, 7)).toBe(0)
    expect(nearestTrendIndex(CHART.right, rect, 7)).toBe(6)
    expect(nearestTrendIndex(CHART.left + CHART_PLOT_WIDTH / 2, rect, 7)).toBe(3)
    expect(nearestTrendIndex(-500, rect, 7)).toBe(0)
    expect(nearestTrendIndex(5000, rect, 7)).toBe(6)
  })

  it('takes the rendered size into account', () => {
    const scaled = { left: 100, width: CHART.width / 2 }
    expect(nearestTrendIndex(100 + CHART.left / 2, scaled, 3)).toBe(0)
    expect(nearestTrendIndex(100 + CHART.right / 2, scaled, 3)).toBe(2)
  })

  it('returns null for missing data or a collapsed chart', () => {
    expect(nearestTrendIndex(100, rect, 0)).toBeNull()
    expect(nearestTrendIndex(100, { left: 0, width: 0 }, 7)).toBeNull()
  })
})

describe('tooltipAnchor', () => {
  it('anchors the tooltip above middle points', () => {
    expect(tooltipAnchor({ x: CHART.left + CHART_PLOT_WIDTH / 2, y: 120 })).toEqual({
      left: expect.closeTo(51.57, 1),
      top: 50,
      align: 'center',
      below: false,
    })
  })

  it('flips horizontally near the edges and vertically near the top', () => {
    expect(tooltipAnchor({ x: CHART.left, y: 120 }).align).toBe('left')
    expect(tooltipAnchor({ x: CHART.right, y: 120 }).align).toBe('right')
    expect(tooltipAnchor({ x: 380, y: CHART.top }).below).toBe(true)
  })
})
