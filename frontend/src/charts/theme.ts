/**
 * 图表明暗主题色板与品牌强调色。
 */
export type ChartMode = 'light' | 'dark'

/**
 * ECharts 画布色板。
 * tooltip 的 DOM 部分仍继承全局样式变量；这里的值用于画布重绘，避免每次读取计算样式。
 */
export interface ChartPalette {
  ink: string
  grid: string
  axis: string
  dot: string
  tooltipBg: string
  tooltipBorder: string
  tooltipShadow: string
  tooltipInset: string
}

/** 浅色/深色画布色板，与 CSS 变量保持同步。 */
const PALETTES: Record<ChartMode, ChartPalette> = {
  light: {
    ink: '#0b1c2c',
    grid: 'rgba(12, 32, 51, 0.07)',
    axis: 'rgba(12, 32, 51, 0.35)',
    dot: '#ffffff',
    tooltipBg: 'rgba(255, 255, 255, 0.68)',
    tooltipBorder: 'rgba(255, 255, 255, 0.85)',
    tooltipShadow: '0 8px 20px rgba(23, 43, 63, 0.1)',
    tooltipInset: 'inset 0 1px 0 #fff',
  },
  dark: {
    ink: '#f3f7fc',
    grid: 'rgba(176, 202, 230, 0.13)',
    axis: 'rgba(190, 211, 233, 0.48)',
    dot: '#101827',
    tooltipBg: 'rgba(19, 27, 41, 0.94)',
    tooltipBorder: 'rgba(154, 180, 210, 0.2)',
    tooltipShadow: '0 12px 28px rgba(0, 0, 0, 0.4)',
    tooltipInset: 'inset 0 1px 0 rgba(255, 255, 255, 0.08)',
  },
}

export function chartPalette(mode: ChartMode): ChartPalette {
  return PALETTES[mode]
}

export const ACCENT = {
  from: '#ff9a6c',
  mid: '#ff5f33',
  to: '#ff4d8d',
  area: '#ff6a3d',
} as const

export const CHART_FONT = '"SFMono-Regular", "JetBrains Mono", Menlo, Consolas, monospace'
