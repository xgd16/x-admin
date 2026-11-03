import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, Select, Switch } from 'antd'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { Job, PageResult } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'

interface JobValues {
  name: string
  cron: string
  handler: string
  params: string
  status: 0 | 1
  remark: string
}

export function Jobs() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Job>()
  const [form] = Form.useForm<JobValues>()

  const query = useQuery({
    queryKey: ['jobs', page, pageSize],
    queryFn: () => api.get<PageResult<Job>>('/jobs', { page, page_size: pageSize }),
  })

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: ['jobs'] })

  const saveMutation = useMutation({
    mutationFn: (values: JobValues) =>
      editing ? api.put<Job>(`/jobs/${editing.id}`, values) : api.post<Job>('/jobs', values),
    onSuccess: () => {
      message.success(editing ? '任务已更新' : '任务已创建')
      setOpen(false)
      setEditing(undefined)
      form.resetFields()
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: 0 | 1 }) => api.put(`/jobs/${id}/status`, { status }),
    onSuccess: () => {
      message.success('任务状态已更新')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const runMutation = useMutation({
    mutationFn: (id: number) => api.post(`/jobs/${id}/run`),
    onSuccess: () => {
      message.success('任务已执行')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/jobs/${id}`),
    onSuccess: () => {
      message.success('任务已删除')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const openCreate = () => {
    setEditing(undefined)
    form.resetFields()
    form.setFieldsValue({ status: 1, cron: '0 3 * * *', handler: 'clean_logs', params: '{"days":90}' })
    setOpen(true)
  }

  return (
    <>
      <PageHeader
        title="定时任务"
        sub={`${query.data?.total ?? 0} 个任务 · robfig/cron 五段表达式 · 执行结果写入任务记录`}
        actions={
          can('system:job:create') ? (
            <button className="btn primary" type="button" onClick={openCreate}>
              <Icon name="plus" size={13} /> 新建任务
            </button>
          ) : undefined
        }
      />

      <DataTable<Job>
        storageKey="jobs"
        rowKey="id"
        data={query.data?.list ?? []}
        loading={query.isFetching}
        total={query.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize) }}
        onRefresh={() => void query.refetch()}
        columns={[
          {
            title: '任务',
            key: 'name',
            width: 210,
            render: (_, row) => (
              <div>
                <b style={{ display: 'block', fontSize: 11 }}>{row.name}</b>
                <span className="mono">{row.remark || row.handler}</span>
              </div>
            ),
          },
          { title: 'Cron', dataIndex: 'cron', key: 'cron', width: 130, render: (value: string) => <span className="mono">{value}</span> },
          {
            title: '状态',
            dataIndex: 'status',
            key: 'status',
            width: 90,
            render: (value: 0 | 1, row) => (
              <Switch
                size="small"
                checked={value === 1}
                loading={statusMutation.isPending && statusMutation.variables?.id === row.id}
                onChange={(checked) => statusMutation.mutate({ id: row.id, status: checked ? 1 : 0 })}
                disabled={!can('system:job:update')}
              />
            ),
          },
          { title: '上次执行', dataIndex: 'last_run_at', key: 'last_run_at', width: 150, render: (value: string | null) => <span className="mono">{value ? dayjs(value).format('MM-DD HH:mm') : '—'}</span> },
          {
            title: '操作',
            key: 'actions',
            width: 180,
            fixed: 'right',
            render: (_, row) => (
              <div className="inline-actions">
                {can('system:job:run') && <Button size="small" type="text" loading={runMutation.isPending && runMutation.variables === row.id} onClick={() => runMutation.mutate(row.id)}>执行</Button>}
                {can('system:job:update') && <Button size="small" type="text" onClick={() => { setEditing(row); form.setFieldsValue(row); setOpen(true) }}>编辑</Button>}
                {can('system:job:delete') && (
                  <Button
                    size="small"
                    type="text"
                    danger
                    onClick={() => {
                      modal.confirm({
                        title: `删除任务「${row.name}」？`,
                        okText: '删除',
                        okButtonProps: { danger: true },
                        cancelText: '取消',
                        onOk: () => deleteMutation.mutateAsync(row.id),
                      })
                    }}
                  >
                    删除
                  </Button>
                )}
              </div>
            ),
          },
        ]}
        toolbar={
          can('system:job:create') ? (
            <Button type="primary" onClick={openCreate}>
              <Icon name="plus" size={12} /> 新建
            </Button>
          ) : undefined
        }
      />

      <FormModal
        open={open}
        title={editing ? '编辑任务' : '新建任务'}
        form={form}
        width={600}
        onCancel={() => { setOpen(false); setEditing(undefined) }}
        onSubmit={(values) => saveMutation.mutate(values as unknown as JobValues)}
        confirmLoading={saveMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="name" label="任务名称" rules={[{ required: true, message: '请输入任务名称' }]}>
            <Input placeholder="清理过期日志" />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '启用', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
          <Form.Item name="cron" label="Cron 表达式" rules={[{ required: true, message: '请输入 Cron' }]}>
            <Input placeholder="0 3 * * *" />
          </Form.Item>
          <Form.Item name="handler" label="执行器" rules={[{ required: true, message: '请输入执行器' }]}>
            <Select
              options={[
                { label: 'clean_logs · 清理审计日志', value: 'clean_logs' },
                { label: 'noop · 空任务', value: 'noop' },
              ]}
            />
          </Form.Item>
          <Form.Item name="params" label="参数 JSON" className="full">
            <Input.TextArea rows={3} placeholder='{"days":90}' />
          </Form.Item>
          <Form.Item name="remark" label="备注" className="full">
            <Input placeholder="可读描述" />
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {() => (
              <div className="stat-chip full" style={{ gridColumn: '1 / -1' }}>
                <Icon name="clock" size={12} /> 支持标准五段：分 时 日 月 周，例如 0 3 * * *
              </div>
            )}
          </Form.Item>
        </div>
      </FormModal>
    </>
  )
}
