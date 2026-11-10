/**
 * 系统监控：主机、Go 运行时、数据库连接池与缓存状态。
 *
 * 布局按 12 列网格组织：顶部四张指标卡，中部把实时趋势作为主视觉，
 * 右侧放依赖状态；下方依次是主机/运行时事实与磁盘/网络明细。
 */
import { useQuery } from '@tanstack/react-query'
import { Select } from 'antd'
import { useEffect, useId, useState, type ReactNode } from 'react'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { MonitorOverview } from '../api/types'
import { PageHeader } from '../components/PageHeader'
import { Icon } from '../components/Icon'
import { useToast } from '../hooks/useToast'

type Tone = 'cyan' | 'violet' | 'amber' | 'mint'

const SAMPLING_KEY = 'x-admin.monitor.interval'
const HISTORY_SECONDS = 10 * 60
const SAMPLING_OPTIONS = [2, 3, 4, 5, 6, 7, 8, 9, 10]

/** 读取用户上次选择的采样间隔，非法值回退到 10 秒。 */
function readSamplingInterval(): number {
  try {
    const saved = Number(localStorage.getItem(SAMPLING_KEY))
    if (Number.isInteger(saved) && saved >= 2 && saved <= 10) return saved
  } catch {
    // localStorage 不可用时使用默认值，不影响监控页主体功能。
  }
  return 10
}

/** 清空趋势历史，保证采样间隔变化后 X 轴仍然均匀。 */
function emptyHistory() {
  return { cpu: [] as number[], memory: [] as number[], goroutines: [] as number[] }
}

/** 百分比保留一位小数；异常值直接归零，避免 NaN 污染界面。 */
function percent(value?: number): string {
  if (!value || Number.isNaN(value)) return '0.0%'
  return `${value.toFixed(1)}%`
}

/** 字节数按 1024 进制展示。 */
function formatBytes(value?: number): string {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

/** 把秒数格式化为 天/时/分。 */
function formatDuration(seconds?: number): string {
  if (!seconds || seconds < 0) return '—'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分`
  return `${minutes} 分`
}

/** 使用率对应的颜色等级：越高越警示。 */
function level(value?: number): 'ok' | 'warn' | 'danger' {
  if ((value ?? 0) >= 85) return 'danger'
  if ((value ?? 0) >= 65) return 'warn'
  return 'ok'
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

/** 小型趋势线，用当前文字颜色绘制，不依赖图表库。 */
function Sparkline({ values, max = 100, tone }: { values: number[]; max?: number; tone: Tone }) {
  const gradientID = useId().replace(/:/g, '')
  const safeMax = Math.max(max, 1)
  const points =
    values.length > 1
      ? values
          .map((value, index) => {
            const x = (index / (values.length - 1)) * 100
            const y = 32 - (clamp(value, 0, safeMax) / safeMax) * 32
            return `${x.toFixed(2)},${y.toFixed(2)}`
          })
          .join(' ')
      : ''

  return (
    <svg className={`monitor-spark monitor-tone-${tone}`} viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <linearGradient id={`spark-${gradientID}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="currentColor" stopOpacity="0.22" />
          <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
        </linearGradient>
      </defs>
      {points && <polygon points={`0,32 ${points} 100,32`} fill={`url(#spark-${gradientID})`} />}
      {points && (
        <polyline
          points={points}
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          vectorEffect="non-scaling-stroke"
        />
      )}
    </svg>
  )
}

/** 顶部指标卡：数值 + 趋势/进度 + 一行说明，高度统一。 */
function StatTile({
  tone,
  label,
  icon,
  value,
  detail,
  series,
  max,
  ratio,
}: {
  tone: Tone
  label: string
  icon: string
  value: ReactNode
  detail: ReactNode
  series?: number[]
  max?: number
  ratio?: number
}) {
  return (
    <article className={`monitor-stat monitor-tone-${tone} glass`}>
      <div className="monitor-stat-top">
        <span>{label}</span>
        <span className="monitor-stat-icon">
          <Icon name={icon} size={13} />
        </span>
      </div>
      <div className="monitor-stat-main">
        <b>{value}</b>
        {series ? (
          <Sparkline values={series} max={max} tone={tone} />
        ) : ratio !== undefined ? (
          <span className={`monitor-stat-bar ${level(ratio)}`}>
            <i style={{ width: `${clamp(ratio, 0, 100)}%` }} />
          </span>
        ) : null}
      </div>
      <p className="monitor-stat-detail">{detail}</p>
    </article>
  )
}

/** 双序列实时趋势图，使用 0-100% 固定纵轴。 */
function TrendChart({
  series,
  samples,
  intervalSec,
}: {
  series: Array<{ label: string; tone: Tone; values: number[]; current: number }>
  samples: number
  intervalSec: number
}) {
  const hasData = series.some((item) => item.values.length > 1)
  return (
    <div className="monitor-chart">
      <div className="monitor-chart-legend">
        {series.map((item) => (
          <span key={item.label} className={`monitor-legend monitor-tone-${item.tone}`}>
            <i />
            {item.label}
            <b>{percent(item.current)}</b>
          </span>
        ))}
        <span className="monitor-chart-window">近 10 分钟</span>
      </div>

      <div className="monitor-chart-body">
        <div className="monitor-chart-axis">
          <span>100%</span>
          <span>50%</span>
          <span>0%</span>
        </div>
        <div className="monitor-chart-plot">
          <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
            {series.map((item) =>
              item.values.length > 1 ? (
                <polyline
                  key={item.label}
                  className={`monitor-tone-${item.tone}`}
                  points={item.values
                    .map((value, index) => {
                      const x = (index / (item.values.length - 1)) * 100
                      const y = 100 - clamp(value, 0, 100)
                      return `${x.toFixed(2)},${y.toFixed(2)}`
                    })
                    .join(' ')}
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  vectorEffect="non-scaling-stroke"
                />
              ) : null,
            )}
          </svg>
          {!hasData && <span className="monitor-chart-empty">正在采集样本…</span>}
        </div>
      </div>

      <div className="monitor-chart-foot">
        <span>{samples > 0 ? `${samples} 个采样点` : '等待首个样本'}</span>
        <span>每 {intervalSec} 秒</span>
      </div>
    </div>
  )
}

/** 事实行：标签在上、值在下，避免密集的边框列表。 */
function Fact({ label, value, full, mono }: { label: string; value: ReactNode; full?: boolean; mono?: boolean }) {
  const empty = value === undefined || value === null || value === ''
  return (
    <div className={`monitor-fact${full ? ' full' : ''}`}>
      <dt>{label}</dt>
      <dd className={mono ? 'mono' : undefined} title={typeof value === 'string' ? value : undefined}>
        {empty ? '—' : value}
      </dd>
    </div>
  )
}

/** 依赖状态行。 */
function ServiceRow({
  name,
  status,
  latency,
  detail,
}: {
  name: string
  status: string
  latency?: number
  detail: ReactNode
}) {
  const up = status === 'up'
  return (
    <div className="monitor-service">
      <span className={`monitor-dot ${up ? 'ok' : 'bad'}`} />
      <div className="monitor-service-main">
        <b>{name}</b>
        <span>{detail}</span>
      </div>
      <div className="monitor-service-meta">
        <b>{latency !== undefined ? `${latency.toFixed(1)}ms` : '—'}</b>
        <em className={up ? 'ok' : 'bad'}>{up ? '正常' : '异常'}</em>
      </div>
    </div>
  )
}

export function Monitor() {
  const toast = useToast()
  const [intervalSec, setIntervalSec] = useState(readSamplingInterval)
  const query = useQuery({
    queryKey: ['monitor-overview'],
    queryFn: () => api.get<MonitorOverview>('/monitor/overview'),
    refetchInterval: intervalSec * 1000,
    staleTime: intervalSec * 1000,
  })

  // 历史窗口固定为 10 分钟；采样越快，保留的点数越多。
  const maxPoints = Math.max(30, Math.round(HISTORY_SECONDS / intervalSec))

  // 前端保留最近 10 分钟的采样，用于绘制实时趋势。
  const [history, setHistory] = useState(emptyHistory)

  useEffect(() => {
    if (!query.data) return
    const next = query.data
    setHistory((current) => {
      const append = (values: number[], value: number) =>
        [...values, Number.isFinite(value) ? value : 0].slice(-maxPoints)
      return {
        cpu: append(current.cpu, next.system.cpu.usage_percent),
        memory: append(current.memory, next.system.memory.used_percent),
        goroutines: append(current.goroutines, next.runtime.goroutines),
      }
    })
    // dataUpdatedAt 在每次成功请求后递增；即使服务端返回的结构完全相同，
    // React Query 的结构共享也可能复用 data 引用，不能只依赖 data 触发采样。
  }, [query.data, query.dataUpdatedAt, maxPoints])

  // 采样间隔变化后清空旧样本，避免不同间隔的点被等距绘制。
  const changeInterval = (seconds: number) => {
    if (seconds === intervalSec) return
    setIntervalSec(seconds)
    setHistory(emptyHistory())
    try {
      localStorage.setItem(SAMPLING_KEY, String(seconds))
    } catch {
      // 持久化失败不影响本次会话。
    }
    void query.refetch()
  }

  // Toast 只在状态切换时提示一次，避免轮询持续失败时刷屏。
  useEffect(() => {
    if (query.isError && query.error) {
      toast.fail(query.error as Error)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.isError])

  const data = query.data
  const system = data?.system
  const host = system?.host
  const cpu = system?.cpu
  const memory = system?.memory
  const runtime = data?.runtime
  const database = data?.database
  const cache = data?.cache
  const disks = system?.disks ?? []
  const diskTotals = disks.reduce(
    (total, disk) => ({
      total: total.total + disk.total,
      used: total.used + disk.used,
      free: total.free + disk.free,
    }),
    { total: 0, used: 0, free: 0 },
  )
  const diskUsage = diskTotals.total > 0 ? (diskTotals.used / diskTotals.total) * 100 : 0
  const hottestDisk = disks.reduce(
    (worst, disk) => (disk.used_percent > (worst?.used_percent ?? -1) ? disk : worst),
    disks[0],
  )
  const goroutineMax = Math.max(4, ...history.goroutines) * 1.15
  const healthState = !data
    ? 'pending'
    : database?.status === 'up' && cache?.status === 'up'
      ? 'ok'
      : 'bad'

  return (
    <>
      <PageHeader
        title="系统监控"
        sub={
          data
            ? `${host?.hostname || '当前主机'} · ${host?.platform || host?.os || '未知系统'} · 每 ${intervalSec} 秒采样 · 最近一次 ${dayjs(data.system.collected_at).format('HH:mm:ss')}`
            : '正在采集主机与运行时指标'
        }
        actions={
          <>
            <div className="monitor-sampling" title="采样间隔（2-10 秒）">
              <Icon name="clock" size={12} />
              <span>采样</span>
              <Select
                size="small"
                variant="borderless"
                value={intervalSec}
                aria-label="采样间隔"
                popupMatchSelectWidth={false}
                onChange={(value) => changeInterval(Number(value))}
                options={SAMPLING_OPTIONS.map((seconds) => ({ value: seconds, label: `${seconds} 秒` }))}
              />
            </div>
            <button className="btn" type="button" aria-busy={query.isFetching || undefined} onClick={() => void query.refetch()}>
              <Icon name="refresh" size={13} className={query.isFetching ? 'x-spin-icon' : undefined} /> 刷新
            </button>
          </>
        }
      />

      <div className="monitor-dashboard">
        <StatTile
          tone="cyan"
          label="CPU"
          icon="activity"
          value={percent(cpu?.usage_percent)}
          detail={`${cpu?.logical_cores ?? 0} 逻辑核 · 负载 ${(system?.load.load1 ?? 0).toFixed(2)}`}
          series={history.cpu}
          max={100}
        />
        <StatTile
          tone="violet"
          label="内存"
          icon="database"
          value={percent(memory?.used_percent)}
          detail={`${formatBytes(memory?.used)} / ${formatBytes(memory?.total)}`}
          series={history.memory}
          max={100}
        />
        <StatTile
          tone="amber"
          label="磁盘峰值"
          icon="archive"
          value={percent(hottestDisk?.used_percent)}
          detail={
            hottestDisk
              ? `${hottestDisk.mountpoint} · ${formatBytes(hottestDisk.used)} / ${formatBytes(hottestDisk.total)}`
              : '未采集到磁盘信息'
          }
          ratio={hottestDisk?.used_percent}
        />
        <StatTile
          tone="mint"
          label="Go 协程"
          icon="zap"
          value={runtime?.goroutines ?? 0}
          detail={`GOMAXPROCS ${runtime?.gomaxprocs ?? 0} · GC ${runtime?.num_gc ?? 0} 次`}
          series={history.goroutines}
          max={goroutineMax}
        />

        <section className="monitor-panel monitor-span-8 glass">
          <div className="monitor-head">
            <h2>实时负载</h2>
            <span>{intervalSec} 秒采样</span>
          </div>
          <TrendChart
            samples={history.cpu.length}
            intervalSec={intervalSec}
            series={[
              { label: 'CPU', tone: 'cyan', values: history.cpu, current: cpu?.usage_percent ?? 0 },
              { label: '内存', tone: 'violet', values: history.memory, current: memory?.used_percent ?? 0 },
            ]}
          />
        </section>

        <section className="monitor-panel monitor-span-4 glass">
          <div className="monitor-head">
            <h2>服务状态</h2>
            <span>依赖探测</span>
          </div>
          <div className={`monitor-health ${healthState}`}>
            <span className="monitor-health-orb" />
            <div>
              <b>{healthState === 'ok' ? '核心依赖正常' : healthState === 'bad' ? '存在异常依赖' : '正在探测'}</b>
              <span>
                {healthState === 'pending'
                  ? '等待首次探测结果'
                  : `MySQL ${database?.status === 'up' ? '已连接' : '异常'} · 缓存 ${cache?.status === 'up' ? '可用' : '异常'}`}
              </span>
            </div>
          </div>
          <div className="monitor-services">
            <ServiceRow
              name="MySQL"
              status={database?.status ?? 'down'}
              latency={database?.latency_ms}
              detail={`连接 ${database?.open ?? 0} · 使用中 ${database?.in_use ?? 0} · 最大 ${database?.max_open ?? 0}`}
            />
            <ServiceRow
              name="缓存"
              status={cache?.status ?? 'down'}
              latency={cache?.latency_ms}
              detail={`${cache?.driver || '未知驱动'} · 连接池等待 ${database?.wait_count ?? 0} 次`}
            />
            <ServiceRow
              name="Go 进程"
              status="up"
              detail={`PID ${data?.app.pid ?? '—'} · 线程 ${system?.process.threads ?? 0}`}
            />
          </div>
          <div className="monitor-app-line">
            <span>{data?.app.name || 'x-admin'}</span>
            <b>{data?.app.version || 'dev'}</b>
            <em>{data?.app.env || '—'}</em>
          </div>
        </section>

        <section className="monitor-panel monitor-span-6 glass">
          <div className="monitor-head">
            <h2>主机信息</h2>
            <span>{host?.arch || 'HOST'}</span>
          </div>
          <dl className="monitor-facts">
            <Fact label="主机名" value={host?.hostname} />
            <Fact label="操作系统" value={`${host?.platform || host?.os || '—'} ${host?.version || ''}`} />
            <Fact label="内核" value={host?.kernel_version} mono />
            <Fact label="架构" value={host?.arch} mono />
            <Fact label="虚拟化" value={host?.virtualization || '物理机 / 未知'} />
            <Fact label="进程数" value={host?.procs} />
            <Fact label="系统运行" value={formatDuration(host?.uptime_seconds)} />
            <Fact
              label="启动时间"
              value={host?.boot_time ? dayjs.unix(host.boot_time).format('YYYY-MM-DD HH:mm') : '—'}
            />
            <Fact
              label="处理器"
              value={cpu?.model ? `${cpu.model}${cpu.mhz ? ` · ${cpu.mhz.toFixed(0)} MHz` : ''}` : '—'}
              full
            />
          </dl>
        </section>

        <section className="monitor-panel monitor-span-6 glass">
          <div className="monitor-head">
            <h2>Go 运行时</h2>
            <span>{runtime?.go_version || 'RUNTIME'}</span>
          </div>
          <dl className="monitor-facts">
            <Fact label="GOMAXPROCS" value={runtime?.gomaxprocs} />
            <Fact label="堆内存" value={formatBytes(runtime?.heap_alloc)} />
            <Fact label="已分配" value={formatBytes(runtime?.alloc)} />
            <Fact label="系统内存" value={formatBytes(runtime?.sys)} />
            <Fact label="GC 次数" value={runtime?.num_gc} />
            <Fact label="最近 GC" value={runtime?.last_gc ? dayjs(runtime.last_gc).format('MM-DD HH:mm:ss') : '—'} mono />
            <Fact label="最近暂停" value={`${(runtime?.last_gc_pause_ms ?? 0).toFixed(2)} ms`} />
            <Fact label="GC CPU" value={`${((runtime?.gc_cpu_fraction ?? 0) * 100).toFixed(2)}%`} />
            <Fact label="应用运行" value={formatDuration(data?.app.uptime_seconds)} />
            <Fact
              label="启动时间"
              value={data?.app.started_at ? dayjs(data.app.started_at).format('MM-DD HH:mm') : '—'}
            />
          </dl>
        </section>

        <section className="monitor-panel monitor-span-8 glass">
          <div className="monitor-head">
            <h2>磁盘</h2>
            <span>{disks.length} 个挂载点</span>
          </div>
          {disks.length === 0 ? (
            <div className="monitor-empty">未采集到磁盘信息。</div>
          ) : (
            <div className="monitor-disk-table">
              <div className="monitor-disk-head">
                <span>挂载点</span>
                <span>使用率</span>
                <span>已用 / 总量</span>
                <span>可用</span>
              </div>
              <div className="monitor-disk-scroll">
                {disks.map((disk) => (
                  <div className="monitor-disk-row" key={disk.mountpoint}>
                    <div className="monitor-disk-main">
                      <b>{disk.mountpoint}</b>
                      <span>
                        {disk.device} · {disk.fstype || '未知文件系统'}
                      </span>
                    </div>
                    <div className="monitor-disk-meter">
                      <span className={`monitor-bar ${level(disk.used_percent)}`}>
                        <i style={{ width: `${clamp(disk.used_percent, 0, 100)}%` }} />
                      </span>
                      <b>{percent(disk.used_percent)}</b>
                    </div>
                    <span className="monitor-disk-number">
                      {formatBytes(disk.used)} / {formatBytes(disk.total)}
                    </span>
                    <span className="monitor-disk-number">{formatBytes(disk.free)}</span>
                  </div>
                ))}
              </div>
              <div className="monitor-disk-summary">
                <div className="monitor-disk-summary-bar">
                  <i style={{ width: `${clamp(diskUsage, 0, 100)}%` }} />
                </div>
                <div className="monitor-disk-summary-meta">
                  <span>总容量 <b>{formatBytes(diskTotals.total)}</b></span>
                  <span>已用 <b>{formatBytes(diskTotals.used)}</b></span>
                  <span>可用 <b>{formatBytes(diskTotals.free)}</b></span>
                </div>
              </div>
            </div>
          )}
        </section>

        <section className="monitor-panel monitor-span-4 glass">
          <div className="monitor-head">
            <h2>网络与进程</h2>
            <span>{system?.network.interfaces ?? 0} 个网卡</span>
          </div>
          <p className="monitor-subhead">网络累计</p>
          <div className="monitor-pairs">
            <div className="monitor-pair">
              <span>发送</span>
              <b>{formatBytes(system?.network.bytes_sent)}</b>
            </div>
            <div className="monitor-pair">
              <span>接收</span>
              <b>{formatBytes(system?.network.bytes_recv)}</b>
            </div>
            <div className="monitor-pair">
              <span>丢包</span>
              <b>{system?.network.drops_in ?? 0}</b>
            </div>
            <div className="monitor-pair">
              <span>错误</span>
              <b>{system?.network.errors_in ?? 0}</b>
            </div>
          </div>
          <div className="monitor-divider" />
          <p className="monitor-subhead">进程</p>
          <div className="monitor-pairs">
            <div className="monitor-pair">
              <span>RSS</span>
              <b>{formatBytes(system?.process.rss)}</b>
            </div>
            <div className="monitor-pair">
              <span>虚拟内存</span>
              <b>{formatBytes(system?.process.vms)}</b>
            </div>
            <div className="monitor-pair">
              <span>线程</span>
              <b>{system?.process.threads ?? 0}</b>
            </div>
            <div className="monitor-pair">
              <span>PID</span>
              <b>{system?.process.pid ?? '—'}</b>
            </div>
          </div>
          <div className="monitor-divider" />
          <div className="monitor-pairs">
            <div className="monitor-pair">
              <span>采集时间</span>
              <b>{system?.collected_at ? dayjs(system.collected_at).format('HH:mm:ss') : '—'}</b>
            </div>
            <div className="monitor-pair">
              <span>刷新间隔</span>
              <b>10 秒</b>
            </div>
          </div>
        </section>
      </div>
    </>
  )
}
