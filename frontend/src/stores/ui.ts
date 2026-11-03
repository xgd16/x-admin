import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeMode = 'light' | 'dark'
export type Density = 'small' | 'middle' | 'large'

export interface ContextPref {
  group: string
  open: boolean
}

interface UiState {
  theme: ThemeMode
  density: Density
  mock: boolean
  watermark: boolean
  columns: Record<string, string[]>
  contextPref: ContextPref | null
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
  setDensity: (density: Density) => void
  setMock: (mock: boolean) => void
  toggleWatermark: () => void
  setColumns: (key: string, columns: string[]) => void
  setContextPref: (group: string, open: boolean) => void
}

export const useUi = create<UiState>()(
  persist(
    (set, get) => ({
      theme: 'light',
      density: 'middle',
      mock: localStorage.getItem('x-admin.mock') === '1',
      watermark: false,
      columns: {},
      contextPref: null,
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set({ theme: get().theme === 'light' ? 'dark' : 'light' }),
      setDensity: (density) => set({ density }),
      setMock: (mock) => {
        localStorage.setItem('x-admin.mock', mock ? '1' : '0')
        set({ mock })
      },
      toggleWatermark: () => set({ watermark: !get().watermark }),
      setColumns: (key, columns) => set({ columns: { ...get().columns, [key]: columns } }),
      setContextPref: (group, open) => set({ contextPref: { group, open } }),
    }),
    {
      name: 'x-admin.ui',
      partialize: (state) => ({
        theme: state.theme,
        density: state.density,
        watermark: state.watermark,
        columns: state.columns,
      }),
    },
  ),
)
