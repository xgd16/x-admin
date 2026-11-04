/**
 * 二级导航面板与服务健康状态。
 */
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api/client'
import type { DashboardSummary } from '../../api/types'
import { railGroups } from '../../config/nav'
import { useAuth } from '../../stores/auth'
import { Icon } from '../Icon'
import { useContextNav } from './useContextNav'

export function ContextBar() {
  const navigate = useNavigate()
  const can = useAuth((state) => state.can)
  const reduceMotion = useReducedMotion()
  const { active: activeItem, items, open, toggle, isAccount } = useContextNav()
  const moduleMeta = railGroups.find((module) => module.group === activeItem.group)
  const title = isAccount ? '个人中心' : activeItem.group
  const subtitle = isAccount
    ? 'WORKSPACE / ACCOUNT'
    : activeItem.group === '工作台'
      ? 'WORKSPACE / OVERVIEW'
      : `LEVEL 1 / ${moduleMeta?.key.toUpperCase() ?? 'MODULE'}`

  // 二级导航里的数量角标来自首页汇总；没有权限时不请求。
  const summary = useQuery({
    queryKey: ['dashboard-summary'],
    queryFn: () => api.get<DashboardSummary>('/dashboard/summary'),
    enabled: open && can('dashboard:view'),
    staleTime: 60_000,
  })

  // 健康检查接口只测往返延迟；面板打开时每 30 秒刷新一次。
  const health = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const start = performance.now()
      const res = await fetch('/healthz')
      if (!res.ok) throw new Error('unhealthy')
      return Math.round(performance.now() - start)
    },
    enabled: open,
    retry: false,
    refetchInterval: open ? 30_000 : false,
  })

  // 角标文案：文件数超过 999 时压缩成 k 单位，保持侧栏宽度稳定。
  const counts: Record<string, string> = {
    dashboard: 'LIVE',
    users: summary.data ? String(summary.data.users) : '',
    roles: summary.data ? String(summary.data.roles) : '',
    menus: summary.data ? String(summary.data.menus) : '',
    files: summary.data ? (summary.data.files > 999 ? `${Math.round(summary.data.files / 100) / 10}k` : String(summary.data.files)) : '',
    jobs: summary.data ? String(summary.data.jobs) : '',
  }

  // 展开/收起使用宽度与位移组合动画；减少动态效果时直接跳过。
  const transition = reduceMotion
    ? { duration: 0 }
    : { type: 'tween' as const, duration: 0.34, ease: [0.22, 0.8, 0.28, 1] as const }

  return (
    <AnimatePresence initial={false}>
      {open && (
        <motion.aside
          key="context"
          aria-label={`${title}二级导航`}
          className="context glass"
          initial={{ width: 0, paddingLeft: 0, paddingRight: 0, opacity: 0, x: -14, marginRight: -10 }}
          animate={{ width: 222, paddingLeft: 10, paddingRight: 10, opacity: 1, x: 0, marginRight: 0 }}
          exit={{ width: 0, paddingLeft: 0, paddingRight: 0, opacity: 0, x: -14, marginRight: -10 }}
          transition={transition}
        >
          <div className="ctx-inner">
            <div className="ctx-head">
              <div className="ctx-kicker">Workspace</div>
              <strong>{title}</strong>
              <span className="ctx-sub">{subtitle}</span>
              <button type="button" className="ctx-collapse" onClick={toggle} title="收起二级导航" aria-label="收起二级导航">
                <Icon name="chevronLeft" size={14} />
              </button>
            </div>
            <nav className="ctx-nav">
              <div className="ctx-label">{activeItem.group === '工作台' ? 'OVERVIEW' : 'MODULES'}</div>
              {items.map((item, index) => (
                <motion.a
                  key={`${activeItem.group}-${item.key}`}
                  href={item.path}
                  className={`ctx-item ${activeItem.key === item.key ? 'active' : ''}`}
                  initial={reduceMotion ? false : { opacity: 0, x: -8 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.22, delay: index * 0.045, ease: [0.22, 0.8, 0.28, 1] }}
                  onClick={(event) => {
                    event.preventDefault()
                    navigate(item.path)
                  }}
                >
                  {activeItem.key === item.key && (
                    <motion.span
                      layoutId={`ctx-active-${activeItem.group}`}
                      className="ctx-active"
                      aria-hidden="true"
                      transition={
                        reduceMotion
                          ? { duration: 0 }
                          : { type: 'spring', stiffness: 600, damping: 45, mass: 0.6 }
                      }
                    >
                      <span className="ctx-active-dot" />
                    </motion.span>
                  )}
                  <span className="ctx-item-label">{item.label}</span>
                  <span className="count">{counts[item.key] ?? ''}</span>
                </motion.a>
              ))}
            </nav>
            <div className="ctx-foot">
              <span className={`dot ${health.isError ? 'bad' : ''}`} />
              {health.isError ? 'API 异常' : 'API 正常'}
              <span className="lat">{health.isError ? '--' : `${health.data ?? 42}ms`}</span>
            </div>
            <div className="ctx-note">
              <Icon name="shield" size={13} />
              <span>RBAC · AUDIT ENABLED</span>
            </div>
          </div>
        </motion.aside>
      )}
    </AnimatePresence>
  )
}
