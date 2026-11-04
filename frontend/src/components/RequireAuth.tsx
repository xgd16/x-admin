/**
 * 路由守卫：启动登录态检查并拦截未登录访问。
 */
import { useEffect, useRef, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { tokenStore } from '../api/tokens'
import { useAuth } from '../stores/auth'

export function RequireAuth({ children }: { children: ReactNode }) {
  const location = useLocation()
  const status = useAuth((state) => state.status)
  const bootstrap = useAuth((state) => state.bootstrap)
  // 确保一次挂载只恢复一次会话，避免 StrictMode 下重复请求 /auth/me。
  const booted = useRef(false)

  useEffect(() => {
    if (booted.current) return
    booted.current = true
    void bootstrap()
  }, [bootstrap])

  // 会话恢复期间先显示启动占位；没有令牌或已确认匿名则跳转登录页。
  if (status === 'idle' || (status === 'loading' && !tokenStore.access)) {
    return <div className="splash">X-ADMIN / LOADING WORKSPACE…</div>
  }
  if (!tokenStore.access || status === 'anon') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
