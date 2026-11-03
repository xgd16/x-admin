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
  refreshKey: number
  open: (tab: Tab) => void
  activate: (key: string) => void
  close: (key: string) => Tab | null
  closeOthers: (key: string) => Tab | null
  closeLeft: (key: string) => Tab | null
  closeRight: (key: string) => Tab | null
  closeAll: () => Tab | null
  refresh: () => void
  reset: () => void
}

export const useTabs = create<TabsState>()(
  persist(
    (set, get) => ({
      tabs: [],
      active: '',
      refreshKey: 0,
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
      closeOthers(key) {
        const { tabs, active } = get()
        const keep = tabs.find((tab) => tab.key === key)
        if (!keep) return null
        set({ tabs: [keep], active: keep.key })
        return active === keep.key ? null : keep
      },
      closeLeft(key) {
        const { tabs, active } = get()
        const index = tabs.findIndex((tab) => tab.key === key)
        if (index <= 0) return null
        const next = tabs.slice(index)
        const activeRemoved = !next.some((tab) => tab.key === active)
        set({ tabs: next, active: activeRemoved ? key : active })
        return activeRemoved ? next[0] : null
      },
      closeRight(key) {
        const { tabs, active } = get()
        const index = tabs.findIndex((tab) => tab.key === key)
        if (index < 0 || index >= tabs.length - 1) return null
        const next = tabs.slice(0, index + 1)
        const activeRemoved = !next.some((tab) => tab.key === active)
        set({ tabs: next, active: activeRemoved ? key : active })
        return activeRemoved ? next[next.length - 1] : null
      },
      closeAll() {
        const dashboard = get().tabs.find((tab) => tab.key === '/dashboard')
        if (dashboard) {
          set({ tabs: [dashboard], active: dashboard.key })
          return dashboard
        }
        set({ tabs: [], active: '' })
        return null
      },
      refresh() {
        set({ refreshKey: get().refreshKey + 1 })
      },
      reset() {
        set({ tabs: [], active: '' })
      },
    }),
    {
      name: 'x-admin.tabs',
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({ tabs: state.tabs, active: state.active }),
    },
  ),
)
