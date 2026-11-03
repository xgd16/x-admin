import { lazy, Suspense } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { DashboardSummary } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { Icon } from '../components/Icon'
import { MethodTag } from '../components/StatusTag'
import { quickActions } from '../config/nav'

// ECharts only ships with the dashboard, not the login entry bundle.
const LoginTrendChart = lazy(() =>
  import('../components/LoginTrendChart').then((module) => ({ default: module.LoginTrendChart })),
)

const numberFormat = new Intl.NumberFormat('zh-CN')

function formatCompact(value: number): string {
  if (value >= 10000) return `${(value / 10000).toFixed(value >= 100000 ? 0 : 1)}w`
  return numberFormat.format(value)
}

function formatBytes(value: number): string {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size.toFixed(1)} ${units[unit]}`
}

export function Dashboard() {
  const navigate = useNavigate()
  const can = useAuth((state) => state.can)
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['dashboard-summary'],
    queryFn: () => api.get<DashboardSummary>('/dashboard/summary'),
    staleTime: 30_000,
  })

  const trend = data?.login_trend ?? []
  const peak = trend.reduce((best, point) => (point.count > best.count ? point : best), { date: '', count: 0 })
  const last = trend[trend.length - 1]
  const prev = trend[trend.length - 2]
  const delta = prev && prev.count > 0 ? ((last.count - prev.count) / prev.count) * 100 : 0

  return (
    <>
      <PageHeader
        title="系统总览"
        sub={
          <>
            {dayjs().format('YYYY-MM-DD HH:mm')} · <b>所有服务正常</b> · 数据每 60s 刷新
          </>
        }
        actions={
          <>
            <button className="btn" type="button" onClick={() => void refetch()}>
              <Icon name="refresh" size={13} />
              刷新
            </button>
            {can('system:user:create') && (
              <button className="btn primary" type="button" onClick={() => navigate('/system/users?new=1')}>
                <Icon name="plus" size={13} />
                新建用户
              </button>
            )}
          </>
        }
      />

      <div className="kpi-grid">
        <article className="kpi glass blue">
          <div className="kpi-top">
            <span className="kpi-label">用户总数</span>
            <span className="kpi-icon">
              <Icon name="users" size={14} />
            </span>
          </div>
          <div className="kpi-value">{data ? numberFormat.format(data.users) : '—'}</div>
          <div className="kpi-foot">
            <span className="delta up">↑ 12.5%</span> 较上月
          </div>
        </article>
        <article className="kpi glass violet">
          <div className="kpi-top">
            <span className="kpi-label">活跃账号</span>
            <span className="kpi-icon">
              <Icon name="zap" size={14} />
            </span>
          </div>
          <div className="kpi-value">{data ? formatCompact(data.active_users) : '—'}</div>
          <div className="kpi-foot">
            <span className="delta up">↑ 8.2%</span> 较昨日
          </div>
        </article>
        <article className="kpi glass peach">
          <div className="kpi-top">
            <span className="kpi-label">今日登录</span>
            <span className="kpi-icon">
              <Icon name="login" size={14} />
            </span>
          </div>
          <div className="kpi-value">{last ? numberFormat.format(last.count) : '—'}</div>
          <div className="kpi-foot">
            <span className={`delta ${delta >= 0 ? 'up' : 'down'}`}>
              {delta >= 0 ? '↑' : '↓'} {Math.abs(delta).toFixed(1)}%
            </span>{' '}
            较昨日
          </div>
        </article>
        <article className="kpi glass mint">
          <div className="kpi-top">
            <span className="kpi-label">待处理</span>
            <span className="kpi-icon">
              <Icon name="clock" size={14} />
            </span>
          </div>
          <div className="kpi-value">7</div>
          <div className="kpi-foot">
            <span className="delta up">2 项</span> 今日新增
          </div>
        </article>
      </div>

      <div className="grid-main">
        <section className="panel glass">
          <div className="panel-head">
            <h2>登录趋势</h2>
            <span>7D / DAILY</span>
          </div>
          <Suspense fallback={<div className="chart chart-skeleton" aria-hidden="true" />}>
            <LoginTrendChart trend={trend} />
          </Suspense>
          <div className="chart-legend">
            <span>
              <i />登录次数
            </span>
            <span className="spacer">{isLoading ? '加载中…' : `近 7 天 · 峰值 ${numberFormat.format(peak.count)}`}</span>
          </div>
        </section>

        <section className="panel glass">
          <div className="panel-head">
            <h2>系统信号</h2>
            <span>4 项 / LIVE</span>
          </div>
          <div className="signal-list">
            <div className="signal">
              <span className="orb" />
              <span>
                <b>账号安全</b>
                <span>登录提醒 · 密码变更通知</span>
              </span>
              <span className="pill">已开启</span>
            </div>
            <div className="signal">
              <span className="orb blue" />
              <span>
                <b>操作审计</b>
                <span>写操作自动留痕</span>
              </span>
              <span className="pill">已开启</span>
            </div>
            <div className="signal">
              <span className="orb amber" />
              <span>
                <b>定时任务</b>
                <span>日志清理 · 每天 03:00</span>
              </span>
              <span className="pill warn">{data?.jobs ?? 0} 个</span>
            </div>
            <div className="signal">
              <span className="orb blue" />
              <span>
                <b>文件存储</b>
                <span>
                  {data?.files ?? 0} 个文件 · {formatBytes(data?.file_size ?? 0)}
                </span>
              </span>
              <span className="pill">本地</span>
            </div>
          </div>
        </section>
      </div>

      <div className="grid-bottom">
        <section className="panel glass">
          <div className="panel-head">
            <h2>最近操作</h2>
            <button className="link" type="button" onClick={() => navigate('/system/logs/operations')}>
              查看全部 →
            </button>
          </div>
          <div style={{ marginTop: 6 }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  {['操作人', '动作', '路径', 'IP', '时间'].map((title) => (
                    <th
                      key={title}
                      style={{
                        textAlign: 'left',
                        padding: '8px 9px',
                        fontFamily: 'var(--mono)',
                        fontSize: 7.5,
                        fontWeight: 400,
                        letterSpacing: '.06em',
                        textTransform: 'uppercase',
                        color: 'var(--dim)',
                        borderBottom: '1px solid var(--line)',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {title}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {(data?.recent_operations ?? []).slice(0, 5).map((row) => (
                  <tr key={row.id}>
                    <td style={{ padding: '9px', fontSize: 10.5, borderBottom: '1px solid var(--line-soft)', whiteSpace: 'nowrap' }}>
                      <div className="cell-user">
                        <span className="cell-avatar">{row.username.slice(0, 1)}</span>
                        <span>
                          <b>{row.username}</b>
                          <small>UID {row.user_id}</small>
                        </span>
                      </div>
                    </td>
                    <td style={{ padding: '9px', borderBottom: '1px solid var(--line-soft)' }}>
                      <MethodTag method={row.method} />
                    </td>
                    <td className="mono" style={{ padding: '9px', borderBottom: '1px solid var(--line-soft)', maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {row.path}
                    </td>
                    <td className="mono" style={{ padding: '9px', borderBottom: '1px solid var(--line-soft)' }}>
                      {row.ip}
                    </td>
                    <td className="mono" style={{ padding: '9px', borderBottom: '1px solid var(--line-soft)' }}>
                      {dayjs(row.created_at).format('MM-DD HH:mm')}
                    </td>
                  </tr>
                ))}
                {(data?.recent_operations ?? []).length === 0 && (
                  <tr>
                    <td colSpan={5} className="empty-hint">
                      暂无操作记录
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section className="panel glass">
          <div className="panel-head">
            <h2>快捷操作</h2>
            <span>COMMANDS</span>
          </div>
          <div className="quick-grid">
            {quickActions.map((action, index) => {
              const colors = [
                ['rgba(255,138,92,.18)', '#b23a12'],
                ['rgba(139,124,246,.16)', '#4f3fc4'],
                ['rgba(47,184,217,.16)', '#0e7490'],
                ['rgba(47,185,138,.16)', '#0c7a55'],
                ['rgba(255,214,153,.3)', '#9a5a0a'],
                ['rgba(47,184,217,.16)', '#0e7490'],
              ]
              return (
                <button
                  key={action.label}
                  type="button"
                  className="quick"
                  onClick={() => navigate(action.path)}
                >
                  <span className="q-icon" style={{ ['--qbg' as string]: colors[index][0], ['--qfg' as string]: colors[index][1] }}>
                    <Icon name={action.icon} size={13} />
                  </span>
                  {action.label}
                  <kbd>{action.hint}</kbd>
                </button>
              )
            })}
          </div>
        </section>
      </div>
    </>
  )
}
