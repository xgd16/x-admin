import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api } from '../api/client'
import { tokenStore } from '../api/tokens'
import type { Captcha, MeResult, TokenPair, User } from '../api/types'

type Status = 'idle' | 'loading' | 'ready' | 'anon'

interface AuthState {
  user: User | null
  permissions: string[]
  status: Status
  demo: boolean
  bootstrap: () => Promise<void>
  login: (username: string, password: string, captchaID: string, code: string) => Promise<void>
  logout: () => Promise<void>
  reset: () => void
  can: (permission?: string) => boolean
}

function isSuper(user: User | null): boolean {
  return !!user?.roles?.some((role) => role.code === 'super_admin')
}

export const useAuth = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      permissions: [],
      status: 'idle',
      demo: false,
      async bootstrap() {
        if (!tokenStore.access) {
          set({ status: 'anon', user: null, permissions: [] })
          return
        }
        set({ status: 'loading' })
        try {
          const me = await api.get<MeResult>('/auth/me')
          set({ user: me.user, permissions: me.permissions, status: 'ready' })
        } catch {
          tokenStore.clear()
          set({ status: 'anon', user: null, permissions: [] })
        }
      },
      async login(username, password, captchaID, code) {
        const pair = await api.post<TokenPair>('/auth/login', {
          username,
          password,
          captcha_id: captchaID,
          captcha_code: code,
        })
        tokenStore.set(pair.access_token, pair.refresh_token)
        const me = await api.get<MeResult>('/auth/me')
        set({ user: me.user, permissions: me.permissions, status: 'ready' })
      },
      async logout() {
        try {
          await api.post('/auth/logout', { refresh_token: tokenStore.refresh })
        } catch {
          // best effort
        }
        tokenStore.clear()
        set({ user: null, permissions: [], status: 'anon', demo: false })
      },
      reset() {
        tokenStore.clear()
        set({ user: null, permissions: [], status: 'anon', demo: false })
      },
      can(permission) {
        if (!permission) return true
        const state = get()
        if (isSuper(state.user)) return true
        return state.permissions.includes(permission)
      },
    }),
    {
      name: 'x-admin.auth',
      partialize: (state) => ({ user: state.user, permissions: state.permissions, status: state.status }),
    },
  ),
)

export function superUser(user: User | null): boolean {
  return isSuper(user)
}

export type { Captcha }
