import { useQuery } from '@tanstack/react-query'
import { useLocation, useNavigate } from 'react-router-dom'
import { itemsOfGroup, navByPath, profileNav, railGroups } from '../../config/nav'
import { api, isMock } from '../../api/client'
import type { DashboardSummary } from '../../api/types'
import { useAuth } from '../../stores/auth'
import { Icon } from '../Icon'

export function ContextBar() {
  const navigate = useNavigate()
  const location = useLocation()
  const can = useAuth((state) => state.can)
  const activeItem = navByPath(location.pathname)
  const moduleMeta = railGroups.find((module) => module.group === activeItem.group)
  const isAccount = activeItem.group === '账号'
  const items = isAccount ? [profileNav] : itemsOfGroup(activeItem.group).filter((item) => can(item.permission))
  const title = isAccount ? '个人中心' : activeItem.group
  const subtitle = isAccount
    ? 'WORKSPACE / ACCOUNT'
    : activeItem.group === '工作台'
      ? 'WORKSPACE / OVERVIEW'
      : `LEVEL 1 / ${moduleMeta?.key.toUpperCase() ?? 'MODULE'}`

  const summary = useQuery({
    queryKey: ['dashboard-summary'],
    queryFn: () => api.get<DashboardSummary>('/dashboard/summary'),
    enabled: can('dashboard:view'),
    staleTime: 60_000,
  })

  const health = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      if (isMock()) return 42
      const start = performance.now()
      const res = await fetch('/healthz')
      if (!res.ok) throw new Error('unhealthy')
      return Math.round(performance.now() - start)
    },
    retry: false,
    refetchInterval: 30_000,
  })

  const counts: Record<string, string> = {
    dashboard: 'LIVE',
    users: summary.data ? String(summary.data.users) : '',
    roles: summary.data ? String(summary.data.roles) : '',
    menus: summary.data ? String(summary.data.menus) : '',
    files: summary.data ? (summary.data.files > 999 ? `${Math.round(summary.data.files / 100) / 10}k` : String(summary.data.files)) : '',
    jobs: summary.data ? String(summary.data.jobs) : '',
  }

  return (
    <aside className="context glass">
      <div className="ctx-head">
        <div className="ctx-kicker">Workspace</div>
        <strong>{title}</strong>
        <span className="ctx-sub">{subtitle}</span>
      </div>
      <nav className="ctx-nav">
        <div className="ctx-label">{activeItem.group === '工作台' ? 'OVERVIEW' : 'MODULES'}</div>
        {items.map((item) => (
          <a
            key={item.key}
            href={item.path}
            className={`ctx-item ${activeItem.key === item.key ? 'active' : ''}`}
            onClick={(event) => {
              event.preventDefault()
              navigate(item.path)
            }}
          >
            <span>{item.label}</span>
            <span className="count">{counts[item.key] ?? ''}</span>
          </a>
        ))}
      </nav>
      <div className="ctx-foot">
        <span className={`dot ${health.isError ? 'bad' : ''}`} />
        {health.isError ? 'API 异常' : isMock() ? 'Mock 正常' : 'API 正常'}
        <span className="lat">{health.isError ? '--' : `${health.data ?? 42}ms`}</span>
      </div>
      <div style={{ display: 'flex', gap: 6, padding: '8px 10px 0', borderTop: '1px solid var(--line)', marginTop: 8 }}>
        <Icon name="shield" size={13} />
        <span style={{ fontFamily: 'var(--mono)', fontSize: 7.5, color: 'var(--dim)' }}>RBAC · AUDIT ENABLED</span>
      </div>
    </aside>
  )
}
