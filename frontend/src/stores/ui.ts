/**
 * 界面偏好 Store：主题、主题色板、表格密度、水印、列设置与二级导航状态。
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
  columns: Record<string, string[]> // 按表格 storageKey 保存用户选择的列。
  contextPref: ContextPref | null // 用户手动展开/收起二级导航的偏好。
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
  setPalette: (palette: ThemePalette) => void
  setDensity: (density: Density) => void
  toggleWatermark: () => void
  setColumns: (key: string, columns: string[]) => void
  setContextPref: (group: string, open: boolean) => void
}

export const useUi = create<UiState>()(
  persist(
    (set, get) => ({
      theme: 'light',
      palette: DEFAULT_PALETTE,
      density: 'middle',
      watermark: false,
      columns: {},
      contextPref: null,
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set({ theme: get().theme === 'light' ? 'dark' : 'light' }),
      setPalette: (palette) => set({ palette }),
      setDensity: (density) => set({ density }),
      toggleWatermark: () => set({ watermark: !get().watermark }),
      setColumns: (key, columns) => set({ columns: { ...get().columns, [key]: columns } }),
      setContextPref: (group, open) => set({ contextPref: { group, open } }),
    }),
    {
      // 仅持久化长期偏好；二级导航状态跟随当前会话即可。
      name: 'x-admin.ui',
      version: 1,
      // 旧版本没有 palette 字段；缺失或非法值统一回退到默认色板。
      migrate: (persisted) => {
        const state = (persisted ?? {}) as Partial<UiState>
        return {
          ...state,
          palette: isThemePalette(state.palette) ? state.palette : DEFAULT_PALETTE,
        } as UiState
      },
      partialize: (state) => ({
        theme: state.theme,
        palette: state.palette,
        density: state.density,
        watermark: state.watermark,
        columns: state.columns,
      }),
    },
  ),
)
