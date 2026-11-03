import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api } from '../api/client'
import { tokenStore } from '../api/tokens'
import type { Captcha, LoginResult, MeResult, PasskeyLoginOptions, User } from '../api/types'
import { getPasskeyAssertion } from '../utils/passkey'

type Status = 'idle' | 'loading' | 'ready' | 'anon'

interface AuthState {
  user: User | null
  permissions: string[]
  status: Status
  demo: boolean
  bootstrap: () => Promise<void>
  login: (username: string, password: string, captchaID: string, code: string) => Promise<LoginResult>
  verifyMFA: (mfaToken: string, code: string) => Promise<void>
  passkeyLogin: () => Promise<void>
  logout: () => Promise<void>
  reset: () => void
  can: (permission?: string) => boolean
}

function isSuper(user: User | null): boolean {
  return !!user?.roles?.some((role) => role.code === 'super_admin')
}

export const useAuth = create<AuthState>()(
  persist(
    (set, get) => {
      const establish = async (result: LoginResult) => {
        if (!result.access_token || !result.refresh_token) {
          throw new Error('登录响应异常，请稍后重试')
        }
        tokenStore.set(result.access_token, result.refresh_token)
        const me = await api.get<MeResult>('/auth/me')
        set({ user: me.user, permissions: me.permissions, status: 'ready' })
      }

      return {
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
          const result = await api.post<LoginResult>('/auth/login', {
            username,
            password,
            captcha_id: captchaID,
            captcha_code: code,
          })
          if (result.mfa_required) {
            if (!result.mfa_token) throw new Error('登录响应异常，请稍后重试')
            return result
          }
          await establish(result)
          return result
        },
        async verifyMFA(mfaToken, code) {
          const result = await api.post<LoginResult>('/auth/login/mfa', {
            mfa_token: mfaToken,
            otp_code: code,
          })
          await establish(result)
        },
        async passkeyLogin() {
          const begin = await api.post<PasskeyLoginOptions>('/auth/passkey/login/begin')
          const credential = await getPasskeyAssertion(begin)
          const result = await api.post<LoginResult>('/auth/passkey/login/finish', {
            session_id: begin.session_id,
            credential,
          })
          await establish(result)
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
      }
    },
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
