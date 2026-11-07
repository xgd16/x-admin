/**
 * 路由守卫：启动登录态检查并拦截未登录访问。
 */
import { useEffect, useRef, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { tokenStore } from '../api/tokens'
import { permissionForPath } from '../config/nav'
import { useAuth } from '../stores/auth'
import { PageLoader } from './Loading'

export function RequireAuth({ children }: { children: ReactNode }) {
  const location = useLocation()
  const status = useAuth((state) => state.status)
  const bootstrap = useAuth((state) => state.bootstrap)
  const can = useAuth((state) => state.can)
  // 确保一次挂载只恢复一次会话，避免 StrictMode 下重复请求 /auth/me。
  const booted = useRef(false)

  useEffect(() => {
    if (booted.current) return
    booted.current = true
    void bootstrap()
  }, [bootstrap])

  // 会话恢复期间先显示启动动画；没有令牌或已确认匿名则跳转登录页。
  if (status === 'idle' || (status === 'loading' && !tokenStore.access)) {
    return <PageLoader />
  }
  if (!tokenStore.access || status === 'anon') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  // 直接输入 URL 也要检查页面权限，不能只依赖导航栏隐藏入口。
  const permission = permissionForPath(location.pathname)
  if (status === 'ready' && permission && !can(permission)) {
    return (
      <div className="splash">
        <b>403</b>
        <span>当前账号没有访问该页面的权限</span>
      </div>
    )
  }
  return <>{children}</>
}
