import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

export interface Tab {
  key: string
  label: string
  path: string
}

interface TabsState {
  tabs: Tab[]
  active: string
  open: (tab: Tab) => void
  activate: (key: string) => void
  close: (key: string) => Tab | null
  reset: () => void
}

export const useTabs = create<TabsState>()(
  persist(
    (set, get) => ({
      tabs: [],
      active: '',
      open(tab) {
        const tabs = get().tabs
        const exists = tabs.find((item) => item.key === tab.key)
        if (!exists) set({ tabs: [...tabs, tab], active: tab.key })
        else set({ active: tab.key })
      },
      activate(key) {
        set({ active: key })
      },
      close(key) {
        const { tabs, active } = get()
        const index = tabs.findIndex((tab) => tab.key === key)
        if (index < 0) return null
        const next = tabs.filter((tab) => tab.key !== key)
        if (next.length === 0) {
          set({ tabs: [], active: '' })
          return null
        }
        const activeTab = next[Math.min(index, next.length - 1)]
        set({ tabs: next, active: active === key ? activeTab.key : active })
        return active === key ? activeTab : null
      },
      reset() {
        set({ tabs: [], active: '' })
      },
    }),
    {
      name: 'x-admin.tabs',
      storage: createJSONStorage(() => sessionStorage),
    },
  ),
)
