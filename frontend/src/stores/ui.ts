/**
 * 界面偏好 Store：主题、主题色板、表格密度、水印、液态玻璃、列设置与二级导航状态。
 */
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { DEFAULT_PALETTE, isThemePalette, type ThemePalette } from '../config/themes'

export type ThemeMode = 'light' | 'dark'
export type Density = 'small' | 'middle' | 'large'

export interface ContextPref {
  group: string
  open: boolean
}

interface UiState {
  theme: ThemeMode
  palette: ThemePalette
  density: Density
  watermark: boolean
  /** 液态玻璃外观：WebGPU 背景 + 玻璃边缘折射，关闭时保持原有玻璃拟态。 */
  liquidGlass: boolean
  /** 闲置自动锁屏分钟数，0 表示关闭。 */
  lockIdleMinutes: number
  columns: Record<string, string[]> // 按表格 storageKey 保存用户选择的列。
  contextPref: ContextPref | null // 用户手动展开/收起二级导航的偏好。
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
  setPalette: (palette: ThemePalette) => void
  setDensity: (density: Density) => void
  toggleWatermark: () => void
  setLiquidGlass: (enabled: boolean) => void
  setLockIdleMinutes: (minutes: number) => void
  setColumns: (key: string, columns: string[]) => void
  setContextPref: (group: string, open: boolean) => void
}

/** 合法的自动锁屏时长档位（分钟），0 为关闭。 */
export const LOCK_IDLE_OPTIONS = [0, 5, 10, 15, 30] as const

export function isLockIdleMinutes(value: unknown): value is number {
  return typeof value === 'number' && LOCK_IDLE_OPTIONS.includes(value as (typeof LOCK_IDLE_OPTIONS)[number])
}

export const useUi = create<UiState>()(
  persist(
    (set, get) => ({
      theme: 'light',
      palette: DEFAULT_PALETTE,
      density: 'middle',
      watermark: false,
      liquidGlass: false,
      lockIdleMinutes: 10,
      columns: {},
      contextPref: null,
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set({ theme: get().theme === 'light' ? 'dark' : 'light' }),
      setPalette: (palette) => set({ palette }),
      setDensity: (density) => set({ density }),
      toggleWatermark: () => set({ watermark: !get().watermark }),
      setLiquidGlass: (enabled) => set({ liquidGlass: enabled }),
      setLockIdleMinutes: (minutes) => set({ lockIdleMinutes: isLockIdleMinutes(minutes) ? minutes : 10 }),
      setColumns: (key, columns) => set({ columns: { ...get().columns, [key]: columns } }),
      setContextPref: (group, open) => set({ contextPref: { group, open } }),
    }),
    {
      // 仅持久化长期偏好；二级导航状态跟随当前会话即可。
      name: 'x-admin.ui',
      version: 3,
      // 旧版本缺少 palette/liquidGlass/lockIdleMinutes 等字段；缺失或非法值统一回退默认值。
      migrate: (persisted) => {
        const state = (persisted ?? {}) as Partial<UiState>
        return {
          ...state,
          palette: isThemePalette(state.palette) ? state.palette : DEFAULT_PALETTE,
          liquidGlass: typeof state.liquidGlass === 'boolean' ? state.liquidGlass : false,
          lockIdleMinutes: isLockIdleMinutes(state.lockIdleMinutes) ? state.lockIdleMinutes : 10,
        } as UiState
      },
      partialize: (state) => ({
        theme: state.theme,
        palette: state.palette,
        density: state.density,
        watermark: state.watermark,
        liquidGlass: state.liquidGlass,
        lockIdleMinutes: state.lockIdleMinutes,
        columns: state.columns,
      }),
    },
  ),
)
