/**
 * 界面材质配置：纯色、毛玻璃与液态玻璃。
 *
 * CSS 通过 <html data-surface="..."> 切换（见 styles/overrides/_solid.scss 与
 * styles/components/_liquid-glass.scss）；纯色为默认值，面向低性能设备。
 */

export type SurfaceMode = 'solid' | 'glass' | 'liquid'

export interface SurfacePreset {
  id: SurfaceMode
  /** 设置菜单里的档位名称。 */
  label: string
  /** 当前档位的简短说明。 */
  description: string
}

export const SURFACES: SurfacePreset[] = [
  {
    id: 'solid',
    label: '纯色',
    description: '实色面板，关闭背景模糊与动画，适合低性能设备',
  },
  {
    id: 'glass',
    label: '毛玻璃',
    description: '半透明磨砂面板，保留极光背景与玻璃高光',
  },
  {
    id: 'liquid',
    label: '液态玻璃',
    description: '玻璃边缘折射与镜面高光，Chromium 系浏览器完整支持',
  },
]

export const DEFAULT_SURFACE: SurfaceMode = 'solid'

export function isSurfaceMode(value: unknown): value is SurfaceMode {
  return SURFACES.some((preset) => preset.id === value)
}
