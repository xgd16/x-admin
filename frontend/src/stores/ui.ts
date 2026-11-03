import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeMode = 'light' | 'dark'
export type Density = 'small' | 'middle' | 'large'

interface UiState {
  theme: ThemeMode
  density: Density
  mock: boolean
  watermark: boolean
  columns: Record<string, string[]>
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
  setDensity: (density: Density) => void
  setMock: (mock: boolean) => void
  toggleWatermark: () => void
  setColumns: (key: string, columns: string[]) => void
}

export const useUi = create<UiState>()(
  persist(
    (set, get) => ({
      theme: 'light',
      density: 'middle',
      mock: localStorage.getItem('x-admin.mock') === '1',
      watermark: false,
      columns: {},
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set({ theme: get().theme === 'light' ? 'dark' : 'light' }),
      setDensity: (density) => set({ density }),
      setMock: (mock) => {
        localStorage.setItem('x-admin.mock', mock ? '1' : '0')
        set({ mock })
      },
      toggleWatermark: () => set({ watermark: !get().watermark }),
      setColumns: (key, columns) => set({ columns: { ...get().columns, [key]: columns } }),
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
