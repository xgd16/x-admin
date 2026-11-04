/**
 * 站内通知弹层：未读轮询、筛选与已读操作。
 */
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Popover, Segmented } from 'antd'
import dayjs from 'dayjs'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api/client'
import type { Notification, PageResult } from '../../api/types'
import { Icon } from '../Icon'

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
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [filter, setFilter] = useState<Filter>('all')

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
    onError: (error: Error) => message.error(error.message),
  })

  const markAllRead = useMutation({
    mutationFn: () => api.put('/notifications/read-all'),
    onSuccess: () => {
      refresh()
      message.success('已全部标记为已读')
    },
    onError: (error: Error) => message.error(error.message),
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
        <button
          type="button"
          className="notif-clear"
          disabled={count === 0 || markAllRead.isPending}
          onClick={() => markAllRead.mutate()}
        >
          全部已读
        </button>
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
          <div className="notif-hint">加载中…</div>
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

  return (
    <Popover
      trigger="click"
      placement="bottomRight"
      open={open}
      onOpenChange={setOpen}
      content={content}
      classNames={{ root: 'notif-popover' }}
    >
      <button
        type="button"
        className="icon-btn"
        title="通知"
        aria-label={count > 0 ? `通知，${count} 条未读` : '通知'}
        aria-expanded={open}
      >
        <Icon name="bell" size={16} />
        {count > 0 && <span className="badge">{count > 99 ? '99+' : count}</span>}
      </button>
    </Popover>
  )
}
