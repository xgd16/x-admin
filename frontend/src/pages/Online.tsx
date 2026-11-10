/**
 * 在线用户：管理员查看全部登录设备并强制下线。
 */
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Input, Tooltip } from 'antd'
import type { TableColumnsType } from 'antd'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { OnlineSession, OnlineSessionPage } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { Icon } from '../components/Icon'
import { useToast } from '../hooks/useToast'

export function Online() {
  const { modal } = App.useApp()
  const toast = useToast()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')

  // 在线列表每 15 秒自动刷新；分页与筛选变化时重新查询。
  const query = useQuery({
    queryKey: ['online-sessions', page, pageSize, keyword],
    queryFn: () => api.get<OnlineSessionPage>('/online-sessions', { page, page_size: pageSize, keyword }),
    refetchInterval: 15_000,
    staleTime: 10_000,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['online-sessions'] })

  const revokeMutation = useMutation({
    mutationFn: ({ userID, sessionID }: { userID: number; sessionID?: string }) =>
      api.del<{ revoked: number }>(`/online-sessions/${userID}`, sessionID ? { session_id: sessionID } : undefined),
    onSuccess: (result) => {
      toast.success(`已下线 ${result.revoked} 个会话`)
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const rows = query.data?.list ?? []

  const columns: TableColumnsType<OnlineSession> = [
    {
      title: '用户',
      key: 'user',
      width: 190,
      render: (_, row) => (
        <div className="cell-user">
          <span className="cell-avatar">{(row.nickname || row.username || '?').slice(0, 1)}</span>
          <span>
            <b>{row.nickname || row.username}</b>
            <small>
              {row.username} · UID {row.user_id}
            </small>
          </span>
        </div>
      ),
    },
    {
      title: '设备',
      key: 'device',
      width: 220,
      render: (_, row) => (
        <div>
          <b style={{ display: 'block', fontSize: 11 }}>{row.device}</b>
          <span className="mono" style={{ display: 'block', maxWidth: 210, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={row.user_agent}>
            {row.user_agent || '未知 User-Agent'}
          </span>
        </div>
      ),
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      key: 'ip',
      width: 140,
      render: (value: string) => <span className="mono">{value || '未知 IP'}</span>,
    },
    {
      title: '登录时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (value: string) => <span className="mono">{dayjs(value).format('MM-DD HH:mm:ss')}</span>,
    },
    {
      title: '最近活动',
      dataIndex: 'last_seen_at',
      key: 'last_seen_at',
      width: 150,
      render: (value: string) => <span className="mono">{dayjs(value).format('MM-DD HH:mm:ss')}</span>,
    },
    {
      title: '状态',
      key: 'status',
      width: 120,
      render: (_, row) => (
        <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {row.current && <span className="tag green">当前会话</span>}
          {row.remember && <span className="tag blue">记住我</span>}
          {!row.current && !row.remember && <span className="tag">普通</span>}
        </div>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      fixed: 'right',
      render: (_, row) => (
        <div className="inline-actions">
          <Tooltip title={row.current ? '当前会话请在个人中心退出' : '该设备需要重新登录'}>
            <Button
              size="small"
              type="text"
              danger
              disabled={row.current || !can('system:online:kick')}
              loading={revokeMutation.isPending && revokeMutation.variables?.sessionID === row.id}
              onClick={() => {
                modal.confirm({
                  title: `强制下线「${row.nickname || row.username}」的当前设备？`,
                  content: `${row.device} · ${row.ip || '未知 IP'}，该设备需要重新登录。`,
                  okText: '强制下线',
                  okButtonProps: { danger: true },
                  cancelText: '取消',
                  onOk: () => revokeMutation.mutateAsync({ userID: row.user_id, sessionID: row.id }),
                })
              }}
            >
              下线
            </Button>
          </Tooltip>
          {can('system:online:kick') && (
            <Button
              size="small"
              type="text"
              onClick={() => {
                modal.confirm({
                  title: `强制下线「${row.nickname || row.username}」的全部设备？`,
                  content: '该用户的所有在线会话都会失效，需要重新登录。',
                  okText: '全部下线',
                  okButtonProps: { danger: true },
                  cancelText: '取消',
                  onOk: () => revokeMutation.mutateAsync({ userID: row.user_id }),
                })
              }}
              loading={revokeMutation.isPending && revokeMutation.variables?.userID === row.user_id && !revokeMutation.variables?.sessionID}
            >
              全部
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="在线用户"
        sub={`${query.data?.users ?? 0} 个在线用户 · ${query.data?.total ?? 0} 个会话 · 每 15s 自动刷新`}
        actions={
          <button className="btn" type="button" aria-busy={query.isFetching || undefined} onClick={() => void query.refetch()}>
            <Icon name="refresh" size={13} className={query.isFetching ? 'x-spin-icon' : undefined} /> 刷新
          </button>
        }
      />

      <DataTable<OnlineSession>
        storageKey="online-sessions"
        rowKey={(row) => `${row.user_id}-${row.id}`}
        data={rows}
        loading={query.isFetching}
        total={query.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => {
          setPage(nextPage)
          setPageSize(nextSize)
        }}
        onRefresh={() => void query.refetch()}
        columns={columns}
        filters={
          <Input
            allowClear
            autoComplete="off"
            placeholder="搜索用户名 / 昵称 / IP / 设备"
            value={keywordDraft}
            onChange={(event) => setKeywordDraft(event.target.value)}
            onPressEnter={() => {
              setKeyword(keywordDraft)
              setPage(1)
            }}
            style={{ minWidth: 240, flex: 1 }}
          />
        }
      />
    </>
  )
}