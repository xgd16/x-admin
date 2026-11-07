/**
 * 部门管理：树形部门 CRUD。
 */
import { useEffect, useState, type Key } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, InputNumber, Select, TreeSelect } from 'antd'
import { api } from '../api/client'
import type { Department } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'
import { StatusTag } from '../components/StatusTag'
import { useToast } from '../hooks/useToast'
import { flattenTree } from '../utils/format'

interface DepartmentValues {
  parent_id: number
  name: string
  leader: string
  phone: string
  email: string
  sort: number
  status: 0 | 1
}

/** 把部门树转换成 TreeSelect 需要的结构。 */
function toTreeSelect(rows: Department[]): Array<{ title: string; value: number; children?: ReturnType<typeof toTreeSelect> }> {
  return rows.map((row) => ({
    title: row.name,
    value: row.id,
    children: row.children ? toTreeSelect(row.children) : undefined,
  }))
}

export function Departments() {
  const { modal } = App.useApp()
  const toast = useToast()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Department>()
  const [expanded, setExpanded] = useState<Key[]>([])
  const [form] = Form.useForm<DepartmentValues>()

  // 部门以树形一次性返回，前端负责展平统计与默认展开。
  const deptsQuery = useQuery({
    queryKey: ['departments-tree'],
    queryFn: () => api.get<Department[]>('/departments/tree'),
  })

  const tree = deptsQuery.data ?? []
  const flat = flattenTree(tree)

  useEffect(() => {
    setExpanded(flat.map((row) => row.id))
  }, [deptsQuery.data])

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: ['departments-tree'] })

  // 新增与编辑共用同一个提交入口，由 editing 是否存在决定请求方法。
  const saveMutation = useMutation({
    mutationFn: (values: DepartmentValues) =>
      editing ? api.put<Department>(`/departments/${editing.id}`, values) : api.post<Department>('/departments', values),
    onSuccess: () => {
      toast.success(editing ? '部门已更新' : '部门已创建')
      setOpen(false)
      setEditing(undefined)
      form.resetFields()
      invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/departments/${id}`),
    onSuccess: () => {
      toast.success('部门已删除')
      invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  // 新建时默认顶级部门、排序 0、状态正常。
  const openCreate = () => {
    setEditing(undefined)
    form.resetFields()
    form.setFieldsValue({ parent_id: 0, sort: 0, status: 1 })
    setOpen(true)
  }

  // 编辑时把整行数据回填到表单。
  const openEdit = (row: Department) => {
    setEditing(row)
    form.setFieldsValue({ ...row })
    setOpen(true)
  }

  const confirmDelete = (row: Department) => {
    modal.confirm({
      title: `删除部门「${row.name}」？`,
      content: '存在子部门或成员时无法删除。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => deleteMutation.mutateAsync(row.id),
    })
  }

  return (
    <>
      <PageHeader
        title="部门管理"
        sub={`${flat.length} 个部门 · 树形组织结构 · 删除前会检查成员`}
        actions={
          can('system:dept:create') ? (
            <button className="btn primary" type="button" onClick={openCreate}>
              <Icon name="plus" size={13} /> 新建部门
            </button>
          ) : undefined
        }
      />

      {/* 树形表格：默认展开全部节点，删除前由后端检查子部门与成员。 */}
      <DataTable<Department>
        storageKey="departments"
        rowKey="id"
        columns={[
          {
            title: '部门名称',
            dataIndex: 'name',
            key: 'name',
            width: 260,
            render: (value: string) => (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span className="cell-avatar" style={{ borderRadius: 8 }}>
                  <Icon name="building" size={12} />
                </span>
                <b style={{ fontSize: 11 }}>{value}</b>
              </div>
            ),
          },
          { title: '负责人', dataIndex: 'leader', key: 'leader', width: 100, render: (value: string) => value || '—' },
          { title: '手机号', dataIndex: 'phone', key: 'phone', width: 130, render: (value: string) => <span className="mono">{value || '—'}</span> },
          { title: '状态', dataIndex: 'status', key: 'status', width: 80, render: (value: 0 | 1) => <StatusTag status={value} /> },
          {
            title: '操作',
            key: 'actions',
            width: 110,
            fixed: 'right',
            render: (_, row) => (
              <div className="inline-actions">
                {can('system:dept:update') && (
                  <Button size="small" type="text" onClick={() => openEdit(row)}>
                    编辑
                  </Button>
                )}
                {can('system:dept:delete') && (
                  <Button size="small" type="text" danger onClick={() => confirmDelete(row)}>
                    删除
                  </Button>
                )}
              </div>
            ),
          },
        ]}
        data={tree}
        loading={deptsQuery.isFetching}
        total={0}
        onRefresh={() => void deptsQuery.refetch()}
        expandable={{
          expandedRowKeys: expanded,
          onExpandedRowsChange: (keys) => setExpanded([...keys]),
        }}
        toolbar={
          can('system:dept:create') ? (
            <Button type="primary" onClick={openCreate}>
              <Icon name="plus" size={12} /> 新建
            </Button>
          ) : undefined
        }
      />

      <FormModal
        open={open}
        title={editing ? '编辑部门' : '新建部门'}
        form={form}
        width={560}
        onCancel={() => {
          setOpen(false)
          setEditing(undefined)
        }}
        onSubmit={(values) => saveMutation.mutate(values as unknown as DepartmentValues)}
        confirmLoading={saveMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="parent_id" label="上级部门">
            <TreeSelect treeData={[{ title: '顶级部门', value: 0, children: toTreeSelect(tree) }]} allowClear placeholder="不选则为顶级" />
          </Form.Item>
          <Form.Item name="name" label="部门名称" rules={[{ required: true, message: '请输入部门名称' }]}>
            <Input autoComplete="off" placeholder="例如：技术部" />
          </Form.Item>
          <Form.Item name="leader" label="负责人">
            <Input autoComplete="name" placeholder="姓名" />
          </Form.Item>
          <Form.Item name="phone" label="手机号">
            <Input autoComplete="tel" placeholder="13800000000" />
          </Form.Item>
          <Form.Item name="email" label="邮箱">
            <Input autoComplete="email" placeholder="name@example.com" />
          </Form.Item>
          <Form.Item name="sort" label="排序" initialValue={0}>
            <InputNumber min={0} max={9999} autoComplete="off" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '正常', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
        </div>
      </FormModal>
    </>
  )
}
