/**
 * 系统监控：主机、Go 运行时、数据库连接池与缓存状态。
 */
import { useQuery } from '@tanstack/react-query'
import { useEffect, type ReactNode } from 'react'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { MonitorOverview } from '../api/types'
import { PageHeader } from '../components/PageHeader'
import { Icon } from '../components/Icon'
import { useToast } from '../hooks/useToast'

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

/** 资源使用率进度条。 */
function Meter({ value, label, detail }: { value?: number; label: string; detail: ReactNode }) {
  const state = level(value)
  return (
    <div className="monitor-meter">
      <div className="monitor-meter-head">
        <b>{label}</b>
        <span className={`mono ${state}`}>{percent(value)}</span>
      </div>
      <div className={`monitor-track ${state}`}>
        <span style={{ width: `${Math.min(100, Math.max(0, value ?? 0))}%` }} />
      </div>
      <div className="monitor-meter-detail">{detail}</div>
    </div>
  )
}

/** 主机信息中的键值行。 */
function InfoRow({ label, value }: { label: string; value: ReactNode }) {
  const empty = value === undefined || value === null || value === ''
  return (
    <div className="monitor-kv">
      <span>{label}</span>
      <b>{empty ? '—' : value}</b>
    </div>
  )
}

/** 依赖状态行。 */
function ServiceRow({ name, status, latency, detail }: { name: string; status: string; latency?: number; detail: ReactNode }) {
  const up = status === 'up'
  return (
    <div className="signal">
      <span className={`orb ${up ? '' : 'red'}`} />
      <span>
        <b>{name}</b>
        <span>{detail}</span>
      </span>
      <span className={`pill ${up ? '' : 'warn'}`}>
        {up ? '正常' : '异常'}
        {latency !== undefined ? ` · ${latency.toFixed(1)}ms` : ''}
      </span>
    </div>
  )
}

export function Monitor() {
  const toast = useToast()
  const query = useQuery({
    queryKey: ['monitor-overview'],
    queryFn: () => api.get<MonitorOverview>('/monitor/overview'),
    refetchInterval: 10_000,
    staleTime: 5_000,
  })

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
  const hottestDisk = disks.reduce((worst, disk) => (disk.used_percent > (worst?.used_percent ?? -1) ? disk : worst), disks[0])

  return (
    <>
      <PageHeader
        title="系统监控"
        sub={
          data
            ? `${host?.hostname || '当前主机'} · ${host?.platform || host?.os || '未知系统'} · 每 10s 自动刷新 · 采集于 ${dayjs(data.system.collected_at).format('HH:mm:ss')}`
            : '正在采集主机与运行时指标'
        }
        actions={
          <button className="btn" type="button" aria-busy={query.isFetching || undefined} onClick={() => void query.refetch()}>
            <Icon name="refresh" size={13} className={query.isFetching ? 'x-spin-icon' : undefined} /> 刷新
          </button>
        }
      />

      <div className="kpi-grid">
        <article className="kpi glass blue">
          <div className="kpi-top">
            <span className="kpi-label">CPU 使用率</span>
            <span className="kpi-icon"><Icon name="activity" size={14} /></span>
          </div>
          <div className="kpi-value">{percent(cpu?.usage_percent)}</div>
          <div className="kpi-foot">
            <span className={`delta ${level(cpu?.usage_percent) === 'ok' ? 'up' : 'down'}`}>
              {cpu?.logical_cores ?? 0} 逻辑核
            </span>
            {cpu?.model || 'CPU 型号未知'}
          </div>
        </article>

        <article className="kpi glass violet">
          <div className="kpi-top">
            <span className="kpi-label">内存使用率</span>
            <span className="kpi-icon"><Icon name="database" size={14} /></span>
          </div>
          <div className="kpi-value">{percent(memory?.used_percent)}</div>
          <div className="kpi-foot">
            <span className="delta up">{formatBytes(memory?.used)}</span> / {formatBytes(memory?.total)}
          </div>
        </article>

        <article className="kpi glass peach">
          <div className="kpi-top">
            <span className="kpi-label">磁盘峰值</span>
            <span className="kpi-icon"><Icon name="archive" size={14} /></span>
          </div>
          <div className="kpi-value">{percent(hottestDisk?.used_percent)}</div>
          <div className="kpi-foot">
            <span className={`delta ${level(hottestDisk?.used_percent) === 'ok' ? 'up' : 'down'}`}>
              {hottestDisk?.mountpoint || '—'}
            </span>
            {formatBytes(hottestDisk?.used)} / {formatBytes(hottestDisk?.total)}
          </div>
        </article>

        <article className="kpi glass mint">
          <div className="kpi-top">
            <span className="kpi-label">Go 协程</span>
            <span className="kpi-icon"><Icon name="zap" size={14} /></span>
          </div>
          <div className="kpi-value">{runtime?.goroutines ?? 0}</div>
          <div className="kpi-foot">
            <span className="delta up">GOMAXPROCS {runtime?.gomaxprocs ?? 0}</span>
            GC {runtime?.num_gc ?? 0} 次
          </div>
        </article>
      </div>

      <div className="grid-main">
        <section className="panel glass">
          <div className="panel-head">
            <h2>资源使用</h2>
            <span>LIVE / 10S</span>
          </div>
          <div className="monitor-meters">
            <Meter
              label="CPU"
              value={cpu?.usage_percent}
              detail={`负载 ${system?.load.load1?.toFixed(2) ?? '0.00'} / ${system?.load.load5?.toFixed(2) ?? '0.00'} / ${system?.load.load15?.toFixed(2) ?? '0.00'}`}
            />
            <Meter
              label="内存"
              value={memory?.used_percent}
              detail={`已用 ${formatBytes(memory?.used)} · 可用 ${formatBytes(memory?.available)} · 共 ${formatBytes(memory?.total)}`}
            />
            <Meter
              label="交换分区"
              value={system?.swap.used_percent}
              detail={
                system?.swap.total
                  ? `已用 ${formatBytes(system.swap.used)} / ${formatBytes(system.swap.total)}`
                  : '未启用交换分区'
              }
            />
            {disks.slice(0, 3).map((disk) => (
              <Meter
                key={disk.mountpoint}
                label={disk.mountpoint}
                value={disk.used_percent}
                detail={`${disk.fstype || '未知文件系统'} · ${formatBytes(disk.used)} / ${formatBytes(disk.total)}`}
              />
            ))}
          </div>
        </section>

        <section className="panel glass">
          <div className="panel-head">
            <h2>依赖状态</h2>
            <span>HEALTH</span>
          </div>
          <div className="signal-list">
            <ServiceRow
              name="MySQL"
              status={database?.status ?? 'down'}
              latency={database?.latency_ms}
              detail={`连接 ${database?.open ?? 0} / 使用中 ${database?.in_use ?? 0} / 最大 ${database?.max_open ?? 0}`}
            />
            <ServiceRow
              name="缓存"
              status={cache?.status ?? 'down'}
              latency={cache?.latency_ms}
              detail={`驱动 ${cache?.driver || '—'} · 连接池等待 ${database?.wait_count ?? 0} 次`}
            />
            <div className="signal">
              <span className="orb blue" />
              <span>
                <b>Go 进程</b>
                <span>PID {data?.app.pid ?? '—'} · 线程 {system?.process.threads ?? 0}</span>
              </span>
              <span className="pill">RSS {formatBytes(system?.process.rss)}</span>
            </div>
            <div className="signal">
              <span className="orb amber" />
              <span>
                <b>应用版本</b>
                <span>{data?.app.name || 'x-admin'} · {data?.app.env || '—'}</span>
              </span>
              <span className="pill">{data?.app.version || 'dev'}</span>
            </div>
          </div>
        </section>
      </div>

      <div className="grid-bottom">
        <section className="panel glass">
          <div className="panel-head">
            <h2>主机信息</h2>
            <span>HOST</span>
          </div>
          <div className="monitor-info-grid">
            <InfoRow label="主机名" value={host?.hostname} />
            <InfoRow label="操作系统" value={`${host?.platform || host?.os || '—'} ${host?.version || ''}`} />
            <InfoRow label="内核" value={host?.kernel_version} />
            <InfoRow label="架构" value={host?.arch} />
            <InfoRow label="虚拟化" value={host?.virtualization || '物理机 / 未知'} />
            <InfoRow label="进程数" value={host?.procs} />
            <InfoRow label="系统运行" value={formatDuration(host?.uptime_seconds)} />
            <InfoRow label="应用运行" value={formatDuration(data?.app.uptime_seconds)} />
            <InfoRow
              label="启动时间"
              value={host?.boot_time ? dayjs.unix(host.boot_time).format('YYYY-MM-DD HH:mm') : '—'}
            />
            <InfoRow label="CPU" value={cpu?.model} />
          </div>
        </section>

        <section className="panel glass">
          <div className="panel-head">
            <h2>Go 运行时</h2>
            <span>{runtime?.go_version || 'RUNTIME'}</span>
          </div>
          <div className="monitor-info-grid">
            <InfoRow label="堆内存" value={formatBytes(runtime?.heap_alloc)} />
            <InfoRow label="已分配" value={formatBytes(runtime?.alloc)} />
            <InfoRow label="系统内存" value={formatBytes(runtime?.sys)} />
            <InfoRow label="堆空闲" value={formatBytes(runtime?.heap_idle)} />
            <InfoRow label="GC 次数" value={runtime?.num_gc} />
            <InfoRow label="最近 GC" value={runtime?.last_gc ? dayjs(runtime.last_gc).format('MM-DD HH:mm:ss') : '—'} />
            <InfoRow label="最近暂停" value={`${(runtime?.last_gc_pause_ms ?? 0).toFixed(2)} ms`} />
            <InfoRow label="GC CPU" value={`${((runtime?.gc_cpu_fraction ?? 0) * 100).toFixed(2)}%`} />
          </div>
        </section>
      </div>

      <section className="panel glass">
        <div className="panel-head">
          <h2>磁盘挂载点</h2>
          <span>{disks.length} 个</span>
        </div>
        <div className="monitor-disks">
          {disks.length === 0 ? (
            <div className="empty-hint">未采集到磁盘信息。</div>
          ) : (
            disks.map((disk) => (
              <div className="monitor-disk" key={disk.mountpoint}>
                <div className="monitor-disk-main">
                  <b>{disk.mountpoint}</b>
                  <span className="mono">{disk.device} · {disk.fstype || '未知文件系统'}</span>
                </div>
                <div className={`monitor-track ${level(disk.used_percent)}`}>
                  <span style={{ width: `${Math.min(100, Math.max(0, disk.used_percent))}%` }} />
                </div>
                <div className="monitor-disk-usage">
                  <b>{percent(disk.used_percent)}</b>
                  <span>{formatBytes(disk.used)} / {formatBytes(disk.total)} · 可用 {formatBytes(disk.free)}</span>
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      <section className="panel glass" style={{ marginTop: 10 }}>
        <div className="panel-head">
          <h2>网络与进程</h2>
          <span>NETWORK</span>
        </div>
        <div className="monitor-info-grid">
          <InfoRow label="网卡数" value={system?.network.interfaces} />
          <InfoRow label="累计发送" value={formatBytes(system?.network.bytes_sent)} />
          <InfoRow label="累计接收" value={formatBytes(system?.network.bytes_recv)} />
          <InfoRow label="丢包 / 错误" value={`${system?.network.drops_in ?? 0} / ${system?.network.errors_in ?? 0}`} />
          <InfoRow label="进程 RSS" value={formatBytes(system?.process.rss)} />
          <InfoRow label="进程虚拟内存" value={formatBytes(system?.process.vms)} />
        </div>
      </section>
    </>
  )
}
