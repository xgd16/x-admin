/**
 * 锁屏 Store：锁定状态跨刷新持久化（防 F5 绕过），解锁由服务端重新校验凭证。
 */
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { ApiError, api } from '../api/client'
import type { PasskeyLoginOptions } from '../api/types'
import { getPasskeyAssertion, passkeyErrorMessage } from '../utils/passkey'

interface LockState {
  locked: boolean
  lockedAt: number
  /** 进入锁屏；lockedAt 供展示与排查，不参与解锁判断。 */
  lock: () => void
  /** 解除锁定（登录、登出或解锁成功后调用）。 */
  reset: () => void
  /** 密码解锁：服务端校验当前用户密码，失败抛出可展示的错误。 */
  unlockWithPassword: (password: string) => Promise<void>
  /** 通行密钥解锁：用户绑定仪式，只接受当前账号自己的凭据。 */
  unlockWithPasskey: () => Promise<void>
}

/** 把解锁过程中的错误转成可直接展示的中文提示。 */
function unlockErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    if (error.code === 40401) return '当前账号还没有绑定通行密钥'
    return error.message || fallback
  }
  return passkeyErrorMessage(error) || fallback
}

export const useLock = create<LockState>()(
  persist(
    (set) => ({
      locked: false,
      lockedAt: 0,
      lock: () => set({ locked: true, lockedAt: Date.now() }),
      reset: () => set({ locked: false, lockedAt: 0 }),
      async unlockWithPassword(password) {
        try {
          await api.post('/auth/unlock', { password })
        } catch (error) {
          throw new Error(unlockErrorMessage(error, '密码验证失败，请重试'))
        }
        set({ locked: false, lockedAt: 0 })
      },
      async unlockWithPasskey() {
        try {
          const begin = await api.post<PasskeyLoginOptions>('/auth/unlock/passkey/begin')
          const credential = await getPasskeyAssertion(begin)
          await api.post('/auth/unlock/passkey/finish', { session_id: begin.session_id, credential })
        } catch (error) {
          throw new Error(unlockErrorMessage(error, '通行密钥验证失败，请重试'))
        }
        set({ locked: false, lockedAt: 0 })
      },
    }),
    {
      // 锁定状态必须跨页面刷新与浏览器重启存活；解锁动作在服务端校验，状态本身不含敏感信息。
      name: 'x-admin.lock',
      partialize: (state) => ({ locked: state.locked, lockedAt: state.lockedAt }),
    },
  ),
)

const LOCK_STORAGE_KEY = 'x-admin.lock'

/** 其他标签页锁定/解锁时同步本标签页；AppLayout 挂载时绑定一次。 */
export function bindLockAcrossTabs(): () => void {
  if (typeof window === 'undefined') return () => {}
  const onStorage = (event: StorageEvent) => {
    if (event.key !== LOCK_STORAGE_KEY || event.newValue == null) return
    try {
      const next = JSON.parse(event.newValue)?.state as { locked?: boolean; lockedAt?: number } | undefined
      const locked = !!next?.locked
      if (locked === useLock.getState().locked) return
      if (locked) useLock.setState({ locked: true, lockedAt: next?.lockedAt ?? Date.now() })
      else useLock.setState({ locked: false, lockedAt: 0 })
    } catch {
      // 无法解析的写入不影响本标签页状态。
    }
  }
  window.addEventListener('storage', onStorage)
  return () => window.removeEventListener('storage', onStorage)
}
