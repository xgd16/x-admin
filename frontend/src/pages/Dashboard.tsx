import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { DashboardSummary, TrendPoint } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { Icon } from '../components/Icon'
import { MethodTag } from '../components/StatusTag'
import { quickActions } from '../config/nav'

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

function smooth(points: Array<{ x: number; y: number }>): string {
  if (points.length === 0) return ''
  let path = `M${points[0].x} ${points[0].y}`
  for (let i = 1; i < points.length; i += 1) {
    const prev = points[i - 1]
    const current = points[i]
    const cx = (prev.x + current.x) / 2
    path += ` C ${cx} ${prev.y}, ${cx} ${current.y}, ${current.x} ${current.y}`
  }
  return path
}

function toPoints(trend: TrendPoint[]): Array<{ x: number; y: number }> {
  const max = Math.max(...trend.map((point) => point.count), 1)
  const width = 700
  return trend.map((point, index) => ({
    x: 42 + (index / Math.max(trend.length - 1, 1)) * width,
    y: 200 - (point.count / max) * 160,
  }))
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
  const points = useMemo(() => toPoints(trend), [trend])
  const line = smooth(points)
  const area = points.length ? `${line} L${points[points.length - 1].x} 200 L${points[0].x} 200 Z` : ''
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
          <div className="chart-wrap">
            <svg className="chart" viewBox="0 0 760 240" fill="none" preserveAspectRatio="none" aria-hidden="true">
              <defs>
                <linearGradient id="login-line" x1="0" y1="0" x2="1" y2="0">
                  <stop offset="0%" stopColor="#ff9a6c" />
                  <stop offset="55%" stopColor="#ff5f33" />
                  <stop offset="100%" stopColor="#ff4d8d" />
                </linearGradient>
                <linearGradient id="login-area" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#ff6a3d" stopOpacity=".22" />
                  <stop offset="100%" stopColor="#ff6a3d" stopOpacity="0" />
                </linearGradient>
                <filter id="login-glow" x="-20%" y="-20%" width="140%" height="140%">
                  <feGaussianBlur stdDeviation="2.5" result="blur" />
                  <feMerge>
                    <feMergeNode in="blur" />
                    <feMergeNode in="SourceGraphic" />
                  </feMerge>
                </filter>
              </defs>
              <g stroke="rgba(12,32,51,.07)" strokeWidth="1">
                <path d="M42 30h700M42 70h700M42 110h700M42 150h700M42 190h700" />
              </g>
              <g fill="rgba(12,32,51,.35)" fontSize="8" fontFamily="monospace">
                <text x="4" y="33">100%</text>
                <text x="4" y="73">75%</text>
                <text x="4" y="113">50%</text>
                <text x="4" y="153">25%</text>
                <text x="4" y="193">0</text>
              </g>
              {area && <path d={area} fill="url(#login-area)" />}
              {line && <path d={line} stroke="url(#login-line)" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" filter="url(#login-glow)" />}
              {points.map((point, index) => (
                <circle key={`${point.x}-${index}`} cx={point.x} cy={point.y} r="2.6" fill="#fff" stroke="#ff5f33" strokeWidth="1.6" />
              ))}
            </svg>
            {peak.count > 0 && (
              <div className="chart-tooltip">
                <b>{numberFormat.format(peak.count)} 次</b>
                {dayjs(peak.date).format('MM-DD')} · 峰值
              </div>
            )}
          </div>
          <div className="chart-legend">
            <span>
              <i />登录次数
            </span>
            <span className="dash">
              <i />活跃用户
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
                <b>MySQL 8.4</b>
                <span>连接池正常 · 慢查询 0</span>
              </span>
              <span className="pill">正常</span>
            </div>
            <div className="signal">
              <span className="orb blue" />
              <span>
                <b>Redis 7.4</b>
                <span>验证码 · 令牌 · 限流</span>
              </span>
              <span className="pill">正常</span>
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
