export type ChartMode = 'light' | 'dark'

/**
 * Canvas colors for ECharts. The DOM parts of the tooltip still inherit
 * `--ink` / `--muted` from global.css; these values mirror the theme so the
 * canvas can repaint without reading computed styles.
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
