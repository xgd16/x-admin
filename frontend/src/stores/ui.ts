/**
 * 界面偏好 Store：主题、材质、主题色板、表格密度、水印、列设置与二级导航状态。
 */
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { DEFAULT_SURFACE, isSurfaceMode, type SurfaceMode } from '../config/appearance'
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
  /** 界面材质：纯色（默认，低性能优先）、毛玻璃、液态玻璃。 */
  surface: SurfaceMode
  /** 闲置自动锁屏分钟数，0 表示关闭。 */
  lockIdleMinutes: number
  columns: Record<string, string[]> // 按表格 storageKey 保存用户选择的列。
  contextPref: ContextPref | null // 用户手动展开/收起二级导航的偏好。
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
  setPalette: (palette: ThemePalette) => void
  setDensity: (density: Density) => void
  toggleWatermark: () => void
  setSurface: (surface: SurfaceMode) => void
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
      surface: DEFAULT_SURFACE,
      lockIdleMinutes: 10,
      columns: {},
      contextPref: null,
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set({ theme: get().theme === 'light' ? 'dark' : 'light' }),
      setPalette: (palette) => set({ palette }),
      setDensity: (density) => set({ density }),
      toggleWatermark: () => set({ watermark: !get().watermark }),
      setSurface: (surface) => set({ surface: isSurfaceMode(surface) ? surface : DEFAULT_SURFACE }),
      setLockIdleMinutes: (minutes) => set({ lockIdleMinutes: isLockIdleMinutes(minutes) ? minutes : 10 }),
      setColumns: (key, columns) => set({ columns: { ...get().columns, [key]: columns } }),
      setContextPref: (group, open) => set({ contextPref: { group, open } }),
    }),
    {
      // 仅持久化长期偏好；二级导航状态跟随当前会话即可。
      name: 'x-admin.ui',
      version: 4,
      // v4 用三档 surface 取代布尔 liquidGlass：显式开启过液态玻璃的老用户保留
      // liquid，其余（旧版默认的 false 和更早版本）统一落到新的纯色默认。
      migrate: (persisted) => {
        const state = (persisted ?? {}) as Partial<UiState> & { liquidGlass?: unknown }
        const legacyLiquid = state.liquidGlass
        const surface = isSurfaceMode(state.surface)
          ? state.surface
          : legacyLiquid === true
            ? 'liquid'
            : DEFAULT_SURFACE
        const { liquidGlass: _legacyLiquidGlass, ...rest } = state
        return {
          ...rest,
          surface,
          palette: isThemePalette(state.palette) ? state.palette : DEFAULT_PALETTE,
          lockIdleMinutes: isLockIdleMinutes(state.lockIdleMinutes) ? state.lockIdleMinutes : 10,
        } as UiState
      },
      partialize: (state) => ({
        theme: state.theme,
        palette: state.palette,
        density: state.density,
        watermark: state.watermark,
        surface: state.surface,
        lockIdleMinutes: state.lockIdleMinutes,
        columns: state.columns,
      }),
    },
  ),
)
