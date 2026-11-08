/**
 * 应用路由表：登录页与受保护的后台页面。
 */
import type { ReactNode } from 'react'
import { App as AntdApp } from 'antd'
import { MotionConfig } from 'motion/react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useNav } from './config/nav'
import { AppLayout } from './components/layout/AppLayout'
import { RequireAuth } from './components/RequireAuth'
import { useIsCompactViewport } from './hooks/useMediaQuery'
import { Dashboard } from './pages/Dashboard'
import { Departments } from './pages/Departments'
import { Dicts } from './pages/Dicts'
import { Files } from './pages/Files'
import { Jobs } from './pages/Jobs'
import { Login } from './pages/Login'
import { Logs } from './pages/Logs'
import { Exports } from './pages/Exports'
import { Menus } from './pages/Menus'
import { NotFound } from './pages/NotFound'
import { Parameters } from './pages/Parameters'
import { Profile } from './pages/Profile'
import { Roles } from './pages/Roles'
import { Users } from './pages/Users'

/** 登录后的默认首页：优先看板，没有权限时跳到第一个可访问页面。 */
function HomeRedirect() {
  const { items } = useNav()
  return <Navigate to={items[0]?.path ?? '/profile'} replace />
}

/**
 * 全局 Toast 作用域：需要路由信息来决定位置，因此挂在 Router 内部。
 * 后台里 Toast 固定在命令栏与页签栏下方，避免遮挡导航；登录页没有框架导航，回到常规顶部。
 */
function ToastScope({ children }: { children: ReactNode }) {
  const location = useLocation()
  const compact = useIsCompactViewport()
  const toastTop = location.pathname === '/login' ? 24 : compact ? 104 : 110
  return (
    <AntdApp
      message={{
        top: toastTop,
        duration: 2.4,
        maxCount: 4,
        stack: { threshold: 3 },
        pauseOnHover: true,
      }}
      notification={{
        placement: compact ? 'top' : 'topRight',
        top: toastTop,
        bottom: 24,
        maxCount: 3,
        duration: 4.5,
        showProgress: true,
        pauseOnHover: true,
        stack: { threshold: 3 },
      }}
    >
      {children}
    </AntdApp>
  )
}

export function AppRoutes() {
  return (
    <ToastScope>
      {/* reducedMotion="user" 会遵循系统的“减少动态效果”设置。 */}
      <MotionConfig reducedMotion="user">
      <Routes>
        <Route path="/login" element={<Login />} />
        {/* 后台页面统一挂载在 AppLayout 下，再由 RequireAuth 做登录校验。 */}
        <Route
          element={
            <RequireAuth>
              <AppLayout />
            </RequireAuth>
          }
        >
          <Route index element={<HomeRedirect />} />
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="system/users" element={<Users />} />
          <Route path="system/roles" element={<Roles />} />
          <Route path="system/menus" element={<Menus />} />
          <Route path="system/departments" element={<Departments />} />
          <Route path="system/dicts" element={<Dicts />} />
          <Route path="system/parameters" element={<Parameters />} />
          <Route path="system/files" element={<Files />} />
          <Route path="system/jobs" element={<Jobs />} />
          <Route path="system/logs/operations" element={<Logs mode="operations" />} />
          <Route path="system/logs/logins" element={<Logs mode="logins" />} />
          <Route path="system/exports" element={<Exports />} />
          <Route path="profile" element={<Profile />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
      </MotionConfig>
    </ToastScope>
  )
}
