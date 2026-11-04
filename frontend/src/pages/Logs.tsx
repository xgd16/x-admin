/**
 * 操作日志与登录日志列表，支持筛选、清空与删除。
 */
import { useState, type Key } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, DatePicker, Descriptions, Drawer, Input, Select, Tag } from 'antd'
import dayjs, { type Dayjs } from 'dayjs'
import { api, saveBlob } from '../api/client'
import type { LoginLog, OperationLog, PageResult } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { Icon } from '../components/Icon'
import { MethodTag } from '../components/StatusTag'

const { RangePicker } = DatePicker

type Mode = 'operations' | 'logins'

/** 按 HTTP 状态码区间着色：5xx 红、4xx 橙、3xx 蓝、其余绿。 */
function statusTag(status: number) {
  const color = status >= 500 ? 'red' : status >= 400 ? 'orange' : status >= 300 ? 'blue' : 'green'
  return <Tag color={color}>{status}</Tag>
}

export function Logs({ mode }: { mode: Mode }) {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')
  const [status, setStatus] = useState<number>()
  const [range, setRange] = useState<[Dayjs, Dayjs] | null>(null)
  const [selected, setSelected] = useState<Key[]>([])
  const [detail, setDetail] = useState<OperationLog | LoginLog | null>(null)

  // 同一个组件复用操作日志与登录日志两套接口和列定义。
  const operations = mode === 'operations'
  const path = operations ? '/logs/operations' : '/logs/logins'
  const filterParams = {
    keyword,
    status,
    start: range?.[0]?.startOf('day').toISOString(),
    end: range?.[1]?.endOf('day').toISOString(),
  }

  const query = useQuery({
    queryKey: [mode, page, pageSize, keyword, status, range?.[0]?.valueOf(), range?.[1]?.valueOf()],
    queryFn: () =>
      api.get<PageResult<OperationLog | LoginLog>>(path, {
        page,
        page_size: pageSize,
        ...filterParams,
      }),
  })

  const clearMutation = useMutation({
    mutationFn: () => api.del(`${path}/clear`),
    onSuccess: () => {
      message.success('日志已清空')
      void queryClient.invalidateQueries({ queryKey: [mode] })
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (ids: number[]) => api.del(path, { ids }),
    onSuccess: () => {
      message.success('日志已删除')
      setSelected([])
      void queryClient.invalidateQueries({ queryKey: [mode] })
    },
    onError: (error: Error) => message.error(error.message),
  })

  const exportLogs = async () => {
    try {
      const blob = await api.download(`${path}/export`, filterParams)
      saveBlob(blob, `${operations ? 'operation' : 'login'}-logs-${Date.now()}.xlsx`)
    } catch (error) {
      message.error(error instanceof Error ? error.message : '导出失败')
    }
  }

  const rows = query.data?.list ?? []

  // 操作日志关注方法/路径/状态，登录日志关注账号/结果/说明。
  const operationColumns = [
    { title: '操作人', key: 'username', width: 130, render: (_: unknown, row: OperationLog) => <b style={{ fontSize: 11 }}>{row.username || `UID ${row.user_id}`}</b> },
    { title: '方法', key: 'method', width: 80, render: (_: unknown, row: OperationLog) => <MethodTag method={row.method} /> },
    { title: '路径', key: 'path', width: 260, render: (_: unknown, row: OperationLog) => <span className="mono">{row.path}</span> },
    { title: 'IP', key: 'ip', width: 120, render: (_: unknown, row: OperationLog) => <span className="mono">{row.ip}</span> },
    { title: '状态', key: 'status', width: 80, render: (_: unknown, row: OperationLog) => statusTag(row.status) },
    { title: '时间', key: 'created_at', width: 150, render: (_: unknown, row: OperationLog) => <span className="mono">{dayjs(row.created_at).format('MM-DD HH:mm:ss')}</span> },
  ]

  const loginColumns = [
    { title: '账号', key: 'username', width: 140, render: (_: unknown, row: LoginLog) => <b style={{ fontSize: 11 }}>{row.username || '—'}</b> },
    { title: '结果', key: 'status', width: 90, render: (_: unknown, row: LoginLog) => statusTag(row.status) },
    { title: '说明', key: 'message', render: (_: unknown, row: LoginLog) => row.message || '—' },
    { title: 'IP', key: 'ip', width: 130, render: (_: unknown, row: LoginLog) => <span className="mono">{row.ip}</span> },
    { title: '时间', key: 'created_at', width: 150, render: (_: unknown, row: LoginLog) => <span className="mono">{dayjs(row.created_at).format('MM-DD HH:mm:ss')}</span> },
  ]

  return (
    <>
      <PageHeader
        title={operations ? '操作日志' : '登录日志'}
        sub={`${query.data?.total ?? 0} 条记录 · ${operations ? '写操作自动审计，密码与令牌已脱敏' : '登录成功与失败都会记录，连续失败 5 次锁定'}`}
        actions={
          can(operations ? 'system:log:operation' : 'system:log:login') ? (
            <>
              <button className="btn" type="button" onClick={() => void exportLogs()}>
                <Icon name="download" size={13} /> 导出
              </button>
              <button
                className="btn"
                type="button"
                onClick={() => {
                  modal.confirm({
                    title: '清空全部日志？',
                    content: '此操作不可恢复，建议先导出备份。',
                    okText: '清空',
                    okButtonProps: { danger: true },
                    cancelText: '取消',
                    onOk: () => clearMutation.mutateAsync(),
                  })
                }}
              >
                <Icon name="trash" size={13} /> 清空
              </button>
            </>
          ) : undefined
        }
      />

      <DataTable<Record<string, unknown>>
        storageKey={mode}
        rowKey="id"
        data={rows as unknown as Array<Record<string, unknown>>}
        loading={query.isFetching}
        total={query.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize) }}
        onRefresh={() => void query.refetch()}
        onRow={(row) => ({ onClick: () => setDetail(row as unknown as OperationLog | LoginLog), style: { cursor: 'pointer' } })}
        columns={(operations ? operationColumns : loginColumns) as never}
        selection={operations ? { selectedRowKeys: selected, onChange: (keys) => setSelected(keys) } : undefined}
        filters={
          <>
            <Input
              allowClear
              placeholder={operations ? '搜索账号 / 路径' : '搜索账号 / IP'}
              value={keywordDraft}
              onChange={(event) => setKeywordDraft(event.target.value)}
              onPressEnter={() => { setKeyword(keywordDraft); setPage(1) }}
              style={{ minWidth: 200, flex: 1 }}
            />
            <Select
              allowClear
              placeholder="全部结果"
              value={status}
              options={(operations ? [200, 201, 400, 401, 403, 404, 500] : [200, 401, 403]).map((value) => ({ label: String(value), value }))}
              onChange={(value) => { setStatus(value); setPage(1) }}
              style={{ width: 120 }}
            />
            <RangePicker
              value={range}
              onChange={(values) => { setRange(values as [Dayjs, Dayjs] | null); setPage(1) }}
              style={{ width: 240 }}
            />
            <Button onClick={() => { setKeyword(keywordDraft); setPage(1) }}>
              <Icon name="search" size={12} /> 筛选
            </Button>
          </>
        }
        toolbar={
          // 只有操作日志支持勾选删除；登录日志保持只读审计。
          operations && can('system:log:operation') ? (
            <Button
              danger
              disabled={selected.length === 0}
              onClick={() => {
                modal.confirm({
                  title: `删除 ${selected.length} 条日志？`,
                  okText: '删除',
                  okButtonProps: { danger: true },
                  cancelText: '取消',
                  onOk: () => deleteMutation.mutateAsync(selected.map(Number)),
                })
              }}
            >
              <Icon name="trash" size={12} /> 删除所选
            </Button>
          ) : (
            <span className="stat-chip">
              <Icon name="shield" size={11} /> 只读审计
            </span>
          )
        }
      />

      <Drawer
        open={!!detail}
        onClose={() => setDetail(null)}
        width={520}
        title={operations ? '操作日志详情' : '登录日志详情'}
      >
        {detail && (
          <Descriptions column={1} size="small" bordered>
            {'message' in detail ? (
              <>
                <Descriptions.Item label="账号">{detail.username || '—'}</Descriptions.Item>
                <Descriptions.Item label="结果">{statusTag(detail.status)}</Descriptions.Item>
                <Descriptions.Item label="说明">{detail.message || '—'}</Descriptions.Item>
                <Descriptions.Item label="IP">{detail.ip || '—'}</Descriptions.Item>
                <Descriptions.Item label="User-Agent">{detail.user_agent || '—'}</Descriptions.Item>
                <Descriptions.Item label="时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              </>
            ) : (
              <>
                <Descriptions.Item label="操作人">{detail.username || `UID ${detail.user_id}`}</Descriptions.Item>
                <Descriptions.Item label="方法">{detail.method}</Descriptions.Item>
                <Descriptions.Item label="路径">{detail.path}</Descriptions.Item>
                <Descriptions.Item label="状态">{statusTag(detail.status)}</Descriptions.Item>
                <Descriptions.Item label="IP">{detail.ip || '—'}</Descriptions.Item>
                <Descriptions.Item label="耗时">{detail.latency_ms} ms</Descriptions.Item>
                <Descriptions.Item label="Request ID">{detail.request_id || '—'}</Descriptions.Item>
                <Descriptions.Item label="User-Agent">{detail.user_agent || '—'}</Descriptions.Item>
                <Descriptions.Item label="请求体">
                  <pre className="mono" style={{ margin: 0, maxHeight: 220, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
                    {detail.body || '—'}
                  </pre>
                </Descriptions.Item>
                <Descriptions.Item label="错误">{detail.error || '—'}</Descriptions.Item>
                <Descriptions.Item label="时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              </>
            )}
          </Descriptions>
        )}
      </Drawer>
    </>
  )
}
