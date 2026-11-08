/**
 * 导出记录：异步导出任务的进度查看、下载、取消与删除。
 */
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Progress, Select, Tag, Tooltip } from 'antd'
import { useNavigate } from 'react-router-dom'
import { api, saveBlob } from '../api/client'
import type { ExportTask, PageResult } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FilenameTemplateDrawer } from '../components/FilenameTemplateDrawer'
import { Icon } from '../components/Icon'
import { useToast } from '../hooks/useToast'
import { formatDate, formatSize } from '../utils/format'

/** 任务状态展示元数据，与后端状态常量一致。 */
const statusMeta: Record<ExportTask['status'], { label: string; color: string }> = {
  0: { label: '排队中', color: 'blue' },
  1: { label: '生成中', color: 'processing' },
  2: { label: '已完成', color: 'green' },
  3: { label: '失败', color: 'red' },
  4: { label: '已取消', color: 'default' },
}

export function Exports() {
  const { modal } = App.useApp()
  const toast = useToast()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [status, setStatus] = useState<ExportTask['status'] | undefined>()
  const [templateOpen, setTemplateOpen] = useState(false)

  const query = useQuery({
    queryKey: ['export-tasks', page, pageSize, status],
    queryFn: () => api.get<PageResult<ExportTask>>('/export-tasks', { page, page_size: pageSize, status }),
    // 存在排队/生成中的任务时每 2 秒轮询，全部结束即停止，避免空转。
    refetchInterval: (q) =>
      (q.state.data?.list ?? []).some((task) => task.status === 0 || task.status === 1) ? 2000 : false,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['export-tasks'] })

  const cancelMutation = useMutation({
    mutationFn: (id: number) => api.post(`/export-tasks/${id}/cancel`),
    onSuccess: () => {
      toast.info('已请求取消，任务将尽快停止')
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (task: ExportTask) => api.del(`/export-tasks/${task.id}`),
    onSuccess: () => {
      toast.success('导出记录已删除')
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const download = (task: ExportTask) => {
    void toast
      .promise(api.download(`/export-tasks/${task.id}/download`).then((blob) => saveBlob(blob, task.file_name)), {
        loading: '正在下载导出文件…',
        success: '导出文件已下载',
        error: (error) => (error instanceof Error ? error.message : '下载失败'),
      })
      .catch(() => undefined)
  }

  const statusColumn = {
    title: '状态',
    key: 'status',
    width: 100,
    render: (_: unknown, task: ExportTask) => {
      const meta = statusMeta[task.status]
      return (
        <Tooltip title={task.error || undefined}>
          <Tag color={meta.color}>{meta.label}</Tag>
        </Tooltip>
      )
    },
  }

  const progressColumn = {
    title: '进度',
    key: 'progress',
    width: 180,
    render: (_: unknown, task: ExportTask) => {
      if (task.status === 2) return <span className="mono">共 {task.processed} 行</span>
      if (task.status === 3 || task.status === 4) {
        return <span className="mono">已处理 {task.processed} 行</span>
      }
      const percent = task.total > 0 ? Math.round((task.processed / task.total) * 100) : 0
      return (
        <div style={{ minWidth: 120 }}>
          <Progress size="small" percent={percent} showInfo={false} />
          <span className="mono" style={{ fontSize: 11 }}>
            {task.processed}/{task.total > 0 ? task.total : '?'} 行
          </span>
        </div>
      )
    },
  }

  const actionColumn = {
    title: '操作',
    key: 'actions',
    width: 150,
    render: (_: unknown, task: ExportTask) => (
      <div style={{ display: 'flex', gap: 4 }}>
        {task.status === 2 && (
          <Button size="small" type="text" onClick={() => download(task)}>
            <Icon name="download" size={12} /> 下载
          </Button>
        )}
        {(task.status === 0 || task.status === 1) && can('system:export:cancel') && (
          <Button size="small" type="text" onClick={() => cancelMutation.mutate(task.id)}>
            取消
          </Button>
        )}
        {task.status !== 0 && task.status !== 1 && can('system:export:delete') && (
          <Button
            size="small"
            type="text"
            danger
            onClick={() => {
              modal.confirm({
                title: '删除该导出记录？',
                content: '对应的导出文件也会一并删除。',
                okText: '删除',
                okButtonProps: { danger: true },
                cancelText: '取消',
                onOk: () => deleteMutation.mutateAsync(task),
              })
            }}
          >
            删除
          </Button>
        )}
      </div>
    ),
  }

  const columns = [
    {
      title: '任务',
      key: 'title',
      render: (_: unknown, task: ExportTask) => (
        <div>
          <b>{task.title}</b>
          <div className="mono" style={{ fontSize: 11, opacity: 0.6 }}>
            {task.module} · #{task.id}
          </div>
        </div>
      ),
    },
    statusColumn,
    progressColumn,
    {
      title: '文件',
      key: 'file',
      width: 190,
      render: (_: unknown, task: ExportTask) =>
        task.file_name ? (
          <span className="mono" style={{ fontSize: 11 }}>
            {task.file_name} ({formatSize(task.file_size)})
          </span>
        ) : (
          '—'
        ),
    },
    {
      title: '创建时间',
      key: 'created_at',
      width: 150,
      render: (_: unknown, task: ExportTask) => <span className="mono">{formatDate(task.created_at)}</span>,
    },
    {
      title: '完成时间',
      key: 'finished_at',
      width: 150,
      render: (_: unknown, task: ExportTask) => <span className="mono">{task.finished_at ? formatDate(task.finished_at) : '—'}</span>,
    },
    actionColumn,
  ]

  const rows = query.data?.list ?? []

  return (
    <>
      <PageHeader
        title="导出记录"
        sub="数据导出在后台分页生成，完成后可在通知中心与本页下载"
        actions={
          <>
            {can('system:export:config') && (
              <button className="btn" type="button" onClick={() => setTemplateOpen(true)}>
                <Icon name="sliders" size={13} /> 文件名配置
              </button>
            )}
            <button
              className="btn"
              type="button"
              onClick={() => navigate('/system/users')}
            >
              <Icon name="users" size={13} /> 去导出用户
            </button>
          </>
        }
      />

      <DataTable<ExportTask>
        storageKey="exports"
        rowKey="id"
        data={rows}
        loading={query.isFetching}
        total={query.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize) }}
        onRefresh={() => void query.refetch()}
        columns={columns}
        filters={
          <Select
            allowClear
            placeholder="全部状态"
            value={status}
            options={Object.entries(statusMeta).map(([value, meta]) => ({ label: meta.label, value: Number(value) }))}
            onChange={(value) => { setStatus(value as ExportTask['status'] | undefined); setPage(1) }}
            style={{ width: 140 }}
          />
        }
        toolbar={
          <span className="stat-chip">
            <Icon name="download" size={11} /> 共 {query.data?.total ?? 0} 条
          </span>
        }
        mobile={{ titleKey: 'title', actionKeys: ['actions'] }}
      />

      <FilenameTemplateDrawer open={templateOpen} onClose={() => setTemplateOpen(false)} />
    </>
  )
}
