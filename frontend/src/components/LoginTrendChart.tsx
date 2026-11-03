import { useEffect, useRef } from 'react'
import { useReducedMotion } from 'motion/react'
import type { EChartsType } from 'echarts/core'
import type { TrendPoint } from '../api/types'
import { useUi } from '../stores/ui'
import { echarts } from '../charts/echarts'
import { chartPalette } from '../charts/theme'
import { buildTrendOption, trendPeakIndex } from '../charts/trendOption'

interface LoginTrendChartProps {
  trend: TrendPoint[]
}

export function LoginTrendChart({ trend }: LoginTrendChartProps) {
  const theme = useUi((state) => state.theme)
  const reduceMotion = useReducedMotion()
  const hostRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<EChartsType | null>(null)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    const chart = echarts.init(host, undefined, { renderer: 'canvas' })
    chartRef.current = chart
    const observer =
      typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => chart.resize())
    observer?.observe(host)
    return () => {
      observer?.disconnect()
      chart.dispose()
      chartRef.current = null
    }
  }, [])

  useEffect(() => {
    chartRef.current?.setOption(
      buildTrendOption(trend, { palette: chartPalette(theme), animate: !reduceMotion }),
    )
  }, [trend, theme, reduceMotion])

  const peak = trendPeakIndex(trend)
  const label = trend.length
    ? `近 ${trend.length} 天登录趋势，峰值 ${trend[peak]?.count ?? 0} 次`
    : '登录趋势暂无数据'

  return (
    <div className="chart-wrap">
      <div className="chart" ref={hostRef} role="img" aria-label={label} />
      {trend.length === 0 && <div className="chart-empty">暂无数据</div>}
    </div>
  )
}
