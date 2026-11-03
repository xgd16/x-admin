import { MotionConfig } from 'motion/react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppLayout } from './components/layout/AppLayout'
import { RequireAuth } from './components/RequireAuth'
import { Dashboard } from './pages/Dashboard'
import { Departments } from './pages/Departments'
import { Dicts } from './pages/Dicts'
import { Files } from './pages/Files'
import { Jobs } from './pages/Jobs'
import { Login } from './pages/Login'
import { Logs } from './pages/Logs'
import { Menus } from './pages/Menus'
import { NotFound } from './pages/NotFound'
import { Parameters } from './pages/Parameters'
import { Profile } from './pages/Profile'
import { Roles } from './pages/Roles'
import { Users } from './pages/Users'

export function AppRoutes() {
  return (
    <MotionConfig reducedMotion="user">
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          element={
            <RequireAuth>
              <AppLayout />
            </RequireAuth>
          }
        >
          <Route index element={<Navigate to="/dashboard" replace />} />
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
          <Route path="profile" element={<Profile />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </MotionConfig>
  )
}
