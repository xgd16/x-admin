import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, Switch } from 'antd'
import { api } from '../api/client'
import type { PageResult, Parameter } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'

interface ParameterValues {
  key: string
  value: string
  name: string
  public: boolean
  remark: string
}

export function Parameters() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Parameter>()
  const [form] = Form.useForm<ParameterValues>()

  const query = useQuery({
    queryKey: ['parameters', page, pageSize, keyword],
    queryFn: () => api.get<PageResult<Parameter>>('/parameters', { page, page_size: pageSize, keyword }),
  })

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: ['parameters'] })

  const saveMutation = useMutation({
    mutationFn: (values: ParameterValues) =>
      editing ? api.put<Parameter>(`/parameters/${editing.id}`, values) : api.post<Parameter>('/parameters', values),
    onSuccess: () => {
      message.success(editing ? '参数已更新' : '参数已创建')
      setOpen(false)
      setEditing(undefined)
      form.resetFields()
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/parameters/${id}`),
    onSuccess: () => {
      message.success('参数已删除')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const openCreate = () => {
    setEditing(undefined)
    form.resetFields()
    form.setFieldsValue({ public: false })
    setOpen(true)
  }

  return (
    <>
      <PageHeader
        title="参数配置"
        sub={`${query.data?.total ?? 0} 个参数 · 公共参数可被未登录页面读取`}
        actions={
          can('system:param:create') ? (
            <button className="btn primary" type="button" onClick={openCreate}>
              <Icon name="plus" size={13} /> 新建参数
            </button>
          ) : undefined
        }
      />

      <DataTable<Parameter>
        storageKey="parameters"
        rowKey="id"
        data={query.data?.list ?? []}
        loading={query.isFetching}
        total={query.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize) }}
        onRefresh={() => void query.refetch()}
        columns={[
          { title: '参数名称', dataIndex: 'name', key: 'name', width: 170, render: (value: string) => <b style={{ fontSize: 11 }}>{value}</b> },
          { title: '参数键', dataIndex: 'key', key: 'key', width: 190, render: (value: string) => <span className="mono">{value}</span> },
          { title: '参数值', dataIndex: 'value', key: 'value', render: (value: string) => <span className="mono" style={{ wordBreak: 'break-all' }}>{value || '—'}</span> },
          { title: '公共', dataIndex: 'public', key: 'public', width: 70, render: (value: boolean) => (value ? '是' : '否') },
          {
            title: '操作',
            key: 'actions',
            width: 110,
            fixed: 'right',
            render: (_, row) => (
              <div className="inline-actions">
                {can('system:param:update') && <Button size="small" type="text" onClick={() => { setEditing(row); form.setFieldsValue(row); setOpen(true) }}>编辑</Button>}
                {can('system:param:delete') && (
                  <Button
                    size="small"
                    type="text"
                    danger
                    onClick={() => {
                      modal.confirm({
                        title: `删除参数「${row.name}」？`,
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
        filters={
          <>
            <Input
              allowClear
              placeholder="搜索参数名称 / 键"
              value={keywordDraft}
              onChange={(event) => setKeywordDraft(event.target.value)}
              onPressEnter={() => { setKeyword(keywordDraft); setPage(1) }}
              style={{ minWidth: 220, flex: 1 }}
            />
            <Button onClick={() => { setKeyword(keywordDraft); setPage(1) }}>
              <Icon name="search" size={12} /> 筛选
            </Button>
          </>
        }
        toolbar={
          can('system:param:create') ? (
            <Button type="primary" onClick={openCreate}>
              <Icon name="plus" size={12} /> 新建
            </Button>
          ) : undefined
        }
      />

      <FormModal
        open={open}
        title={editing ? '编辑参数' : '新建参数'}
        form={form}
        onCancel={() => { setOpen(false); setEditing(undefined) }}
        onSubmit={(values) => saveMutation.mutate(values as unknown as ParameterValues)}
        confirmLoading={saveMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="name" label="参数名称" rules={[{ required: true, message: '请输入参数名称' }]}>
            <Input placeholder="例如：站点名称" />
          </Form.Item>
          <Form.Item name="key" label="参数键" rules={[{ required: true, message: '请输入参数键' }]}>
            <Input placeholder="site.name" />
          </Form.Item>
          <Form.Item name="value" label="参数值" className="full">
            <Input.TextArea rows={3} placeholder="参数值" />
          </Form.Item>
          <Form.Item name="public" label="公共参数" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input placeholder="可选" />
          </Form.Item>
        </div>
      </FormModal>
    </>
  )
}
