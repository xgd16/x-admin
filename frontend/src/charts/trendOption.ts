import dayjs from 'dayjs'
import type { ComposeOption } from 'echarts/core'
import type { LineSeriesOption } from 'echarts/charts'
import type { GridComponentOption, TooltipComponentOption } from 'echarts/components'
import type { TrendPoint } from '../api/types'
import { ACCENT, CHART_FONT, type ChartPalette } from './theme'

export type TrendChartOption = ComposeOption<
  LineSeriesOption | GridComponentOption | TooltipComponentOption
>

const numberFormat = new Intl.NumberFormat('zh-CN')

export function trendPeakIndex(trend: TrendPoint[]): number {
  return trend.reduce((best, point, index, all) => (point.count > all[best].count ? index : best), 0)
}

/** Markup rendered inside the ECharts tooltip; the look lives in global.css. */
export function trendTooltipHtml(trend: TrendPoint[], index: number): string {
  const point = trend[index]
  if (!point) return ''
  const peak = trendPeakIndex(trend)
  const label = index === peak && point.count > 0 ? '峰值' : '当日'
  return `<b>${numberFormat.format(point.count)} 次</b><span>${dayjs(point.date).format('MM-DD')} · ${label}</span>`
}

export interface TrendOptionConfig {
  palette: ChartPalette
  animate?: boolean
}

export function buildTrendOption(
  trend: TrendPoint[],
  { palette, animate = true }: TrendOptionConfig,
): TrendChartOption {
  return {
    animation: animate,
    animationDuration: 760,
    animationEasing: 'cubicOut',
    animationDurationUpdate: 420,
    animationEasingUpdate: 'cubicOut',
    backgroundColor: 'transparent',
    textStyle: { fontFamily: CHART_FONT },
    grid: { left: 4, right: 12, top: 24, bottom: 2, containLabel: true },
    tooltip: {
      trigger: 'axis',
      confine: true,
      enterable: false,
      className: 'chart-tip',
      transitionDuration: 0.12,
      backgroundColor: palette.tooltipBg,
      borderColor: palette.tooltipBorder,
      borderWidth: 1,
      borderRadius: 12,
      padding: [7, 10],
      extraCssText: [
        'backdrop-filter: blur(12px)',
        `box-shadow: ${palette.tooltipShadow}, ${palette.tooltipInset}`,
        `font-family: ${CHART_FONT}`,
        'font-size: 8px',
        'line-height: 1.5',
      ].join(';'),
      textStyle: { color: palette.ink, fontFamily: CHART_FONT, fontSize: 8 },
      axisPointer: {
        type: 'line',
        snap: true,
        lineStyle: { color: 'rgba(255, 95, 51, 0.3)', width: 1, type: 'dashed' },
      },
      formatter: (params: unknown) => {
        const items = Array.isArray(params) ? params : [params]
        const first = items[0] as { dataIndex?: number } | undefined
        return trendTooltipHtml(trend, first?.dataIndex ?? 0)
      },
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: trend.map((point) => point.date),
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: palette.axis,
        fontFamily: CHART_FONT,
        fontSize: 8,
        margin: 10,
        formatter: (value: string) => dayjs(value).format('MM-DD'),
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      min: 0,
      splitNumber: 4,
      minInterval: 1,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: palette.axis,
        fontFamily: CHART_FONT,
        fontSize: 8,
        margin: 10,
        formatter: (value: number) => numberFormat.format(value),
      },
      splitLine: { show: true, lineStyle: { color: palette.grid, width: 1 } },
    },
    series: [
      {
        type: 'line',
        name: '登录次数',
        data: trend.map((point) => point.count),
        smooth: 0.42,
        smoothMonotone: 'x',
        showSymbol: true,
        symbol: 'circle',
        symbolSize: 5.2,
        lineStyle: {
          width: 2.4,
          cap: 'round',
          join: 'round',
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 1,
            y2: 0,
            colorStops: [
              { offset: 0, color: ACCENT.from },
              { offset: 0.55, color: ACCENT.mid },
              { offset: 1, color: ACCENT.to },
            ],
          },
          shadowBlur: 12,
          shadowColor: 'rgba(255, 95, 51, 0.35)',
          shadowOffsetY: 2,
        },
        itemStyle: {
          color: palette.dot,
          borderColor: ACCENT.mid,
          borderWidth: 1.6,
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(255, 106, 61, 0.22)' },
              { offset: 1, color: 'rgba(255, 106, 61, 0)' },
            ],
          },
        },
        emphasis: {
          scale: 1.9,
          itemStyle: {
            color: ACCENT.mid,
            borderColor: '#ffffff',
            borderWidth: 1.4,
            shadowBlur: 12,
            shadowColor: 'rgba(255, 95, 51, 0.45)',
          },
        },
        animationDelay: (index: number) => index * 45,
      },
    ],
  }
}
