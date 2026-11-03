import { useEffect, useRef, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { tokenStore } from '../api/tokens'
import { useAuth } from '../stores/auth'

export function RequireAuth({ children }: { children: ReactNode }) {
  const location = useLocation()
  const status = useAuth((state) => state.status)
  const bootstrap = useAuth((state) => state.bootstrap)
  const booted = useRef(false)

  useEffect(() => {
    if (booted.current) return
    booted.current = true
    void bootstrap()
  }, [bootstrap])

  if (status === 'idle' || (status === 'loading' && !tokenStore.access)) {
    return <div className="splash">X-ADMIN / LOADING WORKSPACE…</div>
  }
  if (!tokenStore.access || status === 'anon') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
