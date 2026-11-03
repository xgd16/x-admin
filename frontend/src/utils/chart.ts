export interface ChartPoint {
  x: number
  y: number
}

export interface TrendLike {
  date: string
  count: number
}

export const CHART = {
  width: 760,
  height: 240,
  left: 42,
  right: 742,
  top: 30,
  baseline: 200,
  plotHeight: 160,
} as const

export const CHART_PLOT_WIDTH = CHART.right - CHART.left

export function trendPoints(trend: TrendLike[]): ChartPoint[] {
  const max = Math.max(...trend.map((point) => point.count), 1)
  return trend.map((point, index) => ({
    x: CHART.left + (index / Math.max(trend.length - 1, 1)) * CHART_PLOT_WIDTH,
    y: CHART.baseline - (point.count / max) * CHART.plotHeight,
  }))
}

export function smoothPath(points: ChartPoint[]): string {
  if (points.length === 0) return ''
  let path = `M${points[0].x} ${points[0].y}`
  for (let i = 1; i < points.length; i += 1) {
    const prev = points[i - 1]
    const current = points[i]
    const cx = (prev.x + current.x) / 2
    path += ` C ${cx} ${prev.y}, ${cx} ${current.y}, ${current.x} ${current.y}`
  }
  return path
}

export function nearestTrendIndex(
  clientX: number,
  rect: { left: number; width: number },
  count: number,
): number | null {
  if (count <= 0 || rect.width <= 0) return null
  const viewX = ((clientX - rect.left) / rect.width) * CHART.width
  const ratio = (viewX - CHART.left) / CHART_PLOT_WIDTH
  const index = Math.round(ratio * Math.max(count - 1, 1))
  return Math.min(Math.max(index, 0), count - 1)
}

export type TooltipAlign = 'left' | 'center' | 'right'

export interface TooltipAnchor {
  left: number
  top: number
  align: TooltipAlign
  below: boolean
}

export function tooltipAnchor(point: ChartPoint): TooltipAnchor {
  const left = (point.x / CHART.width) * 100
  const top = (point.y / CHART.height) * 100
  return {
    left,
    top,
    align: left > 78 ? 'right' : left < 16 ? 'left' : 'center',
    below: top < 30,
  }
}
