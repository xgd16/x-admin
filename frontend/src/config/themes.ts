/**
 * 主题色板配置：品牌强调色、渐变伴随色与浅底深字色。
 *
 * CSS 变量在 src/styles/base/_tokens.scss 中按 data-palette 同步定义，
 * 新增或调整色板时必须两处一起改；图表与 antd token 从这里取值。
 */

export type ThemePalette = 'coral' | 'ocean' | 'forest' | 'violet' | 'rose' | 'graphite'

export interface PalettePreset {
  id: ThemePalette
  /** 中文名，用于偏好设置里的当前色板提示。 */
  label: string
  /** 主色：按钮、选中态、antd colorPrimary。 */
  accent: string
  /** 主色的浅色搭档，用于渐变起点。 */
  accent2: string
  /** 渐变末端/辅色（图表、品牌 loader、进度条）。 */
  accent3: string
  /** 浅色底上的深色文字（头像、图标）。 */
  ink: string
  /** 图表折线渐变起点；缺省用 accent2。 */
  chartFrom?: string
  /** 图表面积填充；缺省用 accent。 */
  chartArea?: string
}

export const PALETTES: PalettePreset[] = [
  {
    id: 'coral',
    label: '珊瑚橙',
    accent: '#ff5f33',
    accent2: '#ff8a5c',
    accent3: '#ff4d8d',
    ink: '#8f2a0d',
    chartFrom: '#ff9a6c',
    chartArea: '#ff6a3d',
  },
  {
    id: 'ocean',
    label: '海洋蓝',
    accent: '#2f7fe0',
    accent2: '#5aa7f0',
    accent3: '#25c2d6',
    ink: '#0d3a6b',
  },
  {
    id: 'forest',
    label: '森野绿',
    accent: '#1f9d6b',
    accent2: '#43c08d',
    accent3: '#9bc53d',
    ink: '#0d4d34',
  },
  {
    id: 'violet',
    label: '暮光紫',
    accent: '#7c5ce6',
    accent2: '#a08bf5',
    accent3: '#e879c9',
    ink: '#3b2a7a',
  },
  {
    id: 'rose',
    label: '玫瑰粉',
    accent: '#e0508f',
    accent2: '#f27ba9',
    accent3: '#b26ae0',
    ink: '#7a1f4d',
  },
  {
    id: 'graphite',
    label: '曜石灰',
    accent: '#4a6076',
    accent2: '#71869c',
    accent3: '#8fa6bb',
    ink: '#22303d',
  },
]

export const DEFAULT_PALETTE: ThemePalette = 'coral'

export function isThemePalette(value: unknown): value is ThemePalette {
  return PALETTES.some((preset) => preset.id === value)
}

/** 未知或缺失的色板 id 回退到默认珊瑚橙。 */
export function paletteOf(id: string | null | undefined): PalettePreset {
  return PALETTES.find((preset) => preset.id === id) ?? PALETTES[0]
}

/** 把十六进制色转成 rgba() 字符串，供 antd token 与 ECharts 使用。 */
export function tint(hex: string, alpha: number): string {
  const value = hex.replace('#', '')
  const r = Number.parseInt(value.slice(0, 2), 16)
  const g = Number.parseInt(value.slice(2, 4), 16)
  const b = Number.parseInt(value.slice(4, 6), 16)
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}
