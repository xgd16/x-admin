/**
 * 登录趋势 ECharts 组件，跟随主题重绘。
 */
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
  const palette = useUi((state) => state.palette)
  const reduceMotion = useReducedMotion()
  const hostRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<EChartsType | null>(null)

  // 图表实例只创建一次；容器尺寸变化时调用 resize，卸载时释放。
  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    const chart = echarts.init(host, undefined, { renderer: 'canvas' })
    chartRef.current = chart
    let resizeTimer = 0
    const observer =
      typeof ResizeObserver === 'undefined'
        ? null
        : new ResizeObserver(() => {
            // 页签保活时隐藏页的容器尺寸为 0，此时 resize 只会白跑一次绘制。
            if (host.clientWidth === 0 || host.clientHeight === 0) {
              window.clearTimeout(resizeTimer)
              return
            }
            // 侧栏展开/收起会让容器连续多帧变化，合并成一次 resize，
            // 避免 ECharts 每一帧都重排画布。
            window.clearTimeout(resizeTimer)
            resizeTimer = window.setTimeout(() => chart.resize(), 160)
          })
    observer?.observe(host)
    return () => {
      window.clearTimeout(resizeTimer)
      observer?.disconnect()
      chart.dispose()
      chartRef.current = null
    }
  }, [])

  // 数据、明暗、色板或“减少动态效果”变化时重新设置 option。
  useEffect(() => {
    chartRef.current?.setOption(
      buildTrendOption(trend, { palette: chartPalette(theme, palette), animate: !reduceMotion }),
    )
  }, [trend, theme, palette, reduceMotion])

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
