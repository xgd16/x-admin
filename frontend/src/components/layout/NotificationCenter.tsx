/**
 * 站内通知弹层：未读轮询、筛选与已读操作。
 */
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Drawer, Popover, Segmented } from 'antd'
import dayjs from 'dayjs'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api/client'
import type { Notification, PageResult } from '../../api/types'
import { useIsCompactViewport } from '../../hooks/useMediaQuery'
import { useToast } from '../../hooks/useToast'
import { Icon } from '../Icon'
import { ListSkeleton, LoadingIndicator } from '../Loading'

type Filter = 'all' | 'unread'

const typeIcon: Record<string, string> = {
  info: 'bell',
  success: 'check',
  warning: 'zap',
  error: 'lock',
}

/** 通知时间按“今天 / 昨天 / 今年 / 更早”逐级简化。 */
function timeLabel(value: string) {
  const at = dayjs(value)
  const today = dayjs()
  if (at.isSame(today, 'day')) return at.format('HH:mm')
  if (at.isSame(today.subtract(1, 'day'), 'day')) return `昨天 ${at.format('HH:mm')}`
  if (at.isSame(today, 'year')) return at.format('MM-DD HH:mm')
  return at.format('YYYY-MM-DD')
}

export function NotificationCenter() {
  const navigate = useNavigate()
  const toast = useToast()
  const queryClient = useQueryClient()
  const compact = useIsCompactViewport()
  const [open, setOpen] = useState(false)
  const [filter, setFilter] = useState<Filter>('all')

  // 切换到桌面布局时关闭手机端底部面板，避免浮层残留在错误位置。
  useEffect(() => {
    if (!compact) setOpen(false)
  }, [compact])

  // 未读数每 60 秒轮询一次，窗口重新聚焦时立即刷新。
  const unread = useQuery({
    queryKey: ['notifications', 'unread'],
    queryFn: () => api.get<{ count: number }>('/notifications/unread-count'),
    refetchInterval: 60_000,
    refetchOnWindowFocus: true,
    staleTime: 20_000,
  })

  // 通知列表只在弹层打开时加载，切换“全部/未读”会重新请求。
  const list = useQuery({
    queryKey: ['notifications', 'list', filter],
    queryFn: () =>
      api.get<PageResult<Notification>>('/notifications', {
        page: 1,
        page_size: 20,
        unread: filter === 'unread' ? 1 : undefined,
      }),
    enabled: open,
    staleTime: 10_000,
    placeholderData: keepPreviousData,
  })

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['notifications'] })
  }

  const markRead = useMutation({
    mutationFn: (id: number) => api.put(`/notifications/${id}/read`),
    onSuccess: refresh,
    onError: (error: Error) => toast.fail(error),
  })

  const markAllRead = useMutation({
    mutationFn: () => api.put('/notifications/read-all'),
    onSuccess: () => {
      refresh()
      toast.success('已全部标记为已读')
    },
    onError: (error: Error) => toast.fail(error),
  })

  const count = unread.data?.count ?? 0
  const rows = list.data?.list ?? []

  // 点击通知时先标记已读，再按后端返回的 link 跳转。
  const openItem = (row: Notification) => {
    if (!row.read_at) markRead.mutate(row.id)
    setOpen(false)
    if (row.link) navigate(row.link)
  }

  const content = (
    <div className="notif-pop">
      <div className="notif-head">
        <div className="notif-heading">
          <strong>通知</strong>
          <span>{count > 0 ? `${count} 条未读` : '暂无未读'}</span>
        </div>
        <div className="notif-actions">
          <button
            type="button"
            className="notif-clear"
            disabled={count === 0 || markAllRead.isPending}
            onClick={() => markAllRead.mutate()}
          >
            {markAllRead.isPending && <LoadingIndicator size="sm" />}
            全部已读
          </button>
          {compact && (
            <button
              type="button"
              className="notif-sheet-close"
              aria-label="关闭通知"
              onClick={() => setOpen(false)}
            >
              <Icon name="x" size={14} />
            </button>
          )}
        </div>
      </div>
      <div className="notif-filter">
        <Segmented
          size="small"
          value={filter}
          options={[
            { label: '全部', value: 'all' },
            { label: '未读', value: 'unread' },
          ]}
          onChange={(value) => setFilter(value as Filter)}
        />
      </div>
      <div className="notif-list">
        {list.isLoading ? (
          <ListSkeleton rows={3} />
        ) : list.isError ? (
          <div className="notif-hint">
            通知加载失败
            <button type="button" onClick={() => void list.refetch()}>
              重试
            </button>
          </div>
        ) : rows.length === 0 ? (
          <div className="notif-empty">
            <Icon name="bell" size={22} />
            <span>{filter === 'unread' ? '没有未读通知' : '暂无通知'}</span>
          </div>
        ) : (
          rows.map((row) => (
            <button
              key={row.id}
              type="button"
              className={`notif-item ${row.read_at ? '' : 'unread'}`}
              onClick={() => openItem(row)}
            >
              <span className={`notif-icon ${row.type}`}>
                <Icon name={typeIcon[row.type] ?? 'bell'} size={13} />
              </span>
              <span className="notif-main">
                <span className="notif-title">
                  <b>{row.title}</b>
                  <i>{timeLabel(row.created_at)}</i>
                </span>
                {row.content && <span className="notif-text">{row.content}</span>}
              </span>
              {!row.read_at && <span className="notif-dot" aria-label="未读" />}
            </button>
          ))
        )}
      </div>
    </div>
  )

  const bell = (
    <button
      type="button"
      className="icon-btn"
      title="通知"
      aria-label={count > 0 ? `通知，${count} 条未读` : '通知'}
      aria-expanded={open}
      onClick={compact ? () => setOpen(true) : undefined}
    >
      <Icon name="bell" size={16} />
      {count > 0 && <span className="badge">{count > 99 ? '99+' : count}</span>}
    </button>
  )

  // 手机面板高度跟随内容，最多占 82% 视口；加载/空态也保留可读高度。
  const contentHeight = rows.reduce((sum, row) => sum + (row.content ? 80 : 58), 0)
  const sheetHeight = Math.min(620, 145 + Math.max(140, contentHeight))

  // 手机端用底部面板展示通知，避免弹层在窄屏被触发器挤到屏幕外。
  if (compact) {
    return (
      <>
        {bell}
        <Drawer
          open={open}
          onClose={() => setOpen(false)}
          placement="bottom"
          size={`min(${sheetHeight}px, 82dvh)`}
          closable={false}
          title={null}
          className="notif-sheet"
          styles={{ body: { padding: 0 } }}
          destroyOnHidden
        >
          <div className="notif-sheet-handle" aria-hidden="true" />
          {content}
        </Drawer>
      </>
    )
  }

  return (
    <Popover
      trigger="click"
      placement="bottomRight"
      open={open}
      onOpenChange={setOpen}
      content={content}
      classNames={{ root: 'notif-popover' }}
    >
      {bell}
    </Popover>
  )
}
