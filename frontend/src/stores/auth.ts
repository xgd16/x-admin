/**
 * 登录态 Store：登录、MFA、通行密钥、登出与权限判断。
 */
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api } from '../api/client'
import { tokenStore } from '../api/tokens'
import type { Captcha, LoginResult, MeResult, PasskeyLoginOptions, User } from '../api/types'
import { getPasskeyAssertion } from '../utils/passkey'

// idle=尚未检查，loading=正在恢复会话，ready=已登录，anon=未登录。
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

/** 超级管理员绕过权限码检查。 */
function isSuper(user: User | null): boolean {
  return !!user?.roles?.some((role) => role.code === 'super_admin')
}

export const useAuth = create<AuthState>()(
  persist(
    (set, get) => {
      // 登录成功后的统一收尾：保存令牌，再拉取用户信息与权限。
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
        // 页面刷新后根据本地令牌恢复会话；令牌失效则回到未登录态。
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
          // 账号开启双重验证时先不落地令牌，等用户提交动态验证码。
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
        // 通行密钥登录：先取 WebAuthn 挑战，再让浏览器签名，最后换令牌。
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
            // 登出失败也不阻塞前端清理，令牌仍会被移除。
          }
          tokenStore.clear()
          set({ user: null, permissions: [], status: 'anon', demo: false })
        },
        reset() {
          tokenStore.clear()
          set({ user: null, permissions: [], status: 'anon', demo: false })
        },
        // 页面与按钮的权限判断入口；空权限码视为公开内容。
        can(permission) {
          if (!permission) return true
          const state = get()
          if (isSuper(state.user)) return true
          return state.permissions.includes(permission)
        },
      }
    },
    {
      // 只持久化用户展示所需字段；令牌单独由 tokenStore 管理。
      name: 'x-admin.auth',
      partialize: (state) => ({ user: state.user, permissions: state.permissions, status: state.status }),
    },
  ),
)

export function superUser(user: User | null): boolean {
  return isSuper(user)
}

export type { Captcha }
