/**
 * 图表明暗画布色板与主题色板强调色。
 */
import { paletteOf, type ThemePalette } from '../config/themes'

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
  /** 折线渐变起点。 */
  accentFrom: string
  /** 主色：折线中段、数据点描边。 */
  accentMid: string
  /** 折线渐变末端。 */
  accentTo: string
  /** 面积填充。 */
  accentArea: string
}

type CanvasPalette = Omit<ChartPalette, 'accentFrom' | 'accentMid' | 'accentTo' | 'accentArea'>

/** 浅色/深色画布中性色，与 CSS 变量保持同步。 */
const CANVAS_PALETTES: Record<ChartMode, CanvasPalette> = {
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

/** 明亮/深浅两种画布 + 当前主题色板的品牌色。 */
export function chartPalette(mode: ChartMode, palette: ThemePalette = 'coral'): ChartPalette {
  const preset = paletteOf(palette)
  return {
    ...CANVAS_PALETTES[mode],
    accentFrom: preset.chartFrom ?? preset.accent2,
    accentMid: preset.accent,
    accentTo: preset.accent3,
    accentArea: preset.chartArea ?? preset.accent,
  }
}

export const CHART_FONT = '"SFMono-Regular", "JetBrains Mono", Menlo, Consolas, monospace'
