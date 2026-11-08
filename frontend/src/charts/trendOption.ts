/**
 * 登录趋势图的 ECharts option 构建。
 */
import dayjs from 'dayjs'
import type { ComposeOption } from 'echarts/core'
import type { LineSeriesOption } from 'echarts/charts'
import type { GridComponentOption, TooltipComponentOption } from 'echarts/components'
import type { TrendPoint } from '../api/types'
import { tint } from '../config/themes'
import { CHART_FONT, type ChartPalette } from './theme'

export type TrendChartOption = ComposeOption<
  LineSeriesOption | GridComponentOption | TooltipComponentOption
>

const numberFormat = new Intl.NumberFormat('zh-CN')

/** 返回登录次数最多那天的下标；空数组或全为 0 时返回 0。 */
export function trendPeakIndex(trend: TrendPoint[]): number {
  return trend.reduce((best, point, index, all) => (point.count > all[best].count ? index : best), 0)
}

/** 生成 tooltip 内部 HTML；具体视觉样式由 styles/index.scss 控制。 */
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

/** 根据趋势数据和当前主题构建折线图配置。 */
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
    // ECharts 6 弃用 containLabel，等价写法为 outerBoundsMode:'same' + outerBoundsContain:'axisLabel'。
    grid: { left: 4, right: 12, top: 24, bottom: 2, outerBoundsMode: 'same', outerBoundsContain: 'axisLabel' },
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
        lineStyle: { color: tint(palette.accentMid, 0.3), width: 1, type: 'dashed' },
      },
      // ECharts 可能传入单个对象或数组，这里统一取第一条数据。
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
    // 珊瑚色渐变折线 + 向下淡出的面积填充。
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
              { offset: 0, color: palette.accentFrom },
              { offset: 0.55, color: palette.accentMid },
              { offset: 1, color: palette.accentTo },
            ],
          },
          shadowBlur: 12,
          shadowColor: tint(palette.accentMid, 0.35),
          shadowOffsetY: 2,
        },
        itemStyle: {
          color: palette.dot,
          borderColor: palette.accentMid,
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
              { offset: 0, color: tint(palette.accentArea, 0.22) },
              { offset: 1, color: tint(palette.accentArea, 0) },
            ],
          },
        },
        emphasis: {
          scale: 1.9,
          itemStyle: {
            color: palette.accentMid,
            borderColor: '#ffffff',
            borderWidth: 1.4,
            shadowBlur: 12,
            shadowColor: tint(palette.accentMid, 0.45),
          },
        },
        animationDelay: (index: number) => index * 45,
      },
    ],
  }
}
