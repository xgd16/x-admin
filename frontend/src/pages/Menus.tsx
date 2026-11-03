import { useEffect, useState, type Key } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, InputNumber, Segmented, Select, Switch, TreeSelect } from 'antd'
import { api } from '../api/client'
import type { Menu } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'
import { StatusTag } from '../components/StatusTag'
import { flattenTree } from '../utils/format'

interface MenuValues {
  parent_id: number
  name: string
  type: 1 | 2 | 3
  path: string
  component: string
  permission: string
  icon: string
  sort: number
  visible: boolean
  status: 0 | 1
}

function toTreeSelect(menus: Menu[]): Array<{ title: string; value: number; children?: ReturnType<typeof toTreeSelect> }> {
  return menus.map((menu) => ({
    title: menu.name,
    value: menu.id,
    children: menu.children ? toTreeSelect(menu.children) : undefined,
  }))
}

const typeLabels: Record<number, string> = { 1: '目录', 2: '菜单', 3: '按钮' }

export function Menus() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Menu>()
  const [expanded, setExpanded] = useState<Key[]>([])
  const [form] = Form.useForm<MenuValues>()
  const type = Form.useWatch('type', form)

  const menusQuery = useQuery({
    queryKey: ['menus-tree'],
    queryFn: () => api.get<Menu[]>('/menus/tree'),
  })

  const tree = menusQuery.data ?? []
  const flat = flattenTree(tree)

  useEffect(() => {
    setExpanded(flat.map((menu) => menu.id))
  }, [menusQuery.data])

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['menus-tree'] })
    void queryClient.invalidateQueries({ queryKey: ['menus'] })
  }

  const saveMutation = useMutation({
    mutationFn: (values: MenuValues) =>
      editing ? api.put<Menu>(`/menus/${editing.id}`, values) : api.post<Menu>('/menus', values),
    onSuccess: () => {
      message.success(editing ? '菜单已更新' : '菜单已创建')
      setOpen(false)
      setEditing(undefined)
      form.resetFields()
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/menus/${id}`),
    onSuccess: () => {
      message.success('菜单已删除')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const openCreate = () => {
    setEditing(undefined)
    form.resetFields()
    form.setFieldsValue({ parent_id: 0, type: 2, sort: 0, visible: true, status: 1 })
    setOpen(true)
  }

  const openEdit = (menu: Menu) => {
    setEditing(menu)
    form.setFieldsValue({ ...menu })
    setOpen(true)
  }

  const confirmDelete = (menu: Menu) => {
    modal.confirm({
      title: `删除菜单「${menu.name}」？`,
      content: '存在子菜单时无法删除，需要先清理子级。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => deleteMutation.mutateAsync(menu.id),
    })
  }

  return (
    <>
      <PageHeader
        title="菜单管理"
        sub={`${flat.length} 个节点 · 目录 / 菜单 / 按钮三级权限 · 修改后重新登录生效`}
        actions={
          can('system:menu:create') ? (
            <button className="btn primary" type="button" onClick={openCreate}>
              <Icon name="plus" size={13} /> 新建菜单
            </button>
          ) : undefined
        }
      />

      <DataTable<Menu>
        storageKey="menus"
        rowKey="id"
        columns={[
          {
            title: '名称',
            dataIndex: 'name',
            key: 'name',
            width: 240,
            render: (value: string, row) => (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span className="cell-avatar" style={{ borderRadius: 8 }}>
                  <Icon name={row.icon || 'file'} size={12} />
                </span>
                <b style={{ fontSize: 11 }}>{value}</b>
              </div>
            ),
          },
          {
            title: '类型',
            dataIndex: 'type',
            key: 'type',
            width: 70,
            render: (value: number) => <span className={`tag ${value === 3 ? 'orange' : value === 2 ? 'green' : ''}`}>{typeLabels[value]}</span>,
          },
          { title: '路径', dataIndex: 'path', key: 'path', width: 170, render: (value: string) => <span className="mono">{value || '—'}</span> },
          {
            title: '权限码',
            dataIndex: 'permission',
            key: 'permission',
            width: 200,
            ellipsis: true,
            render: (value: string) => <span className="mono" title={value}>{value || '—'}</span>,
          },
          { title: '状态', dataIndex: 'status', key: 'status', width: 70, render: (value: 0 | 1) => <StatusTag status={value} /> },
          {
            title: '操作',
            key: 'actions',
            width: 110,
            fixed: 'right',
            render: (_, menu) => (
              <div className="inline-actions">
                {can('system:menu:update') && (
                  <Button size="small" type="text" onClick={() => openEdit(menu)}>
                    编辑
                  </Button>
                )}
                {can('system:menu:delete') && (
                  <Button size="small" type="text" danger onClick={() => confirmDelete(menu)}>
                    删除
                  </Button>
                )}
              </div>
            ),
          },
        ]}
        data={tree}
        loading={menusQuery.isFetching}
        total={0}
        onRefresh={() => void menusQuery.refetch()}
        expandable={{
          expandedRowKeys: expanded,
          onExpandedRowsChange: (keys) => setExpanded([...keys]),
        }}
        toolbar={
          can('system:menu:create') ? (
            <Button type="primary" onClick={openCreate}>
              <Icon name="plus" size={12} /> 新建
            </Button>
          ) : undefined
        }
      />

      <FormModal
        open={open}
        title={editing ? '编辑菜单' : '新建菜单'}
        form={form}
        width={600}
        onCancel={() => {
          setOpen(false)
          setEditing(undefined)
        }}
        onSubmit={(values) => saveMutation.mutate(values as unknown as MenuValues)}
        confirmLoading={saveMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="parent_id" label="上级菜单">
            <TreeSelect
              treeData={[{ title: '顶级菜单', value: 0, children: toTreeSelect(tree) }]}
              placeholder="不选则为顶级"
              allowClear
            />
          </Form.Item>
          <Form.Item name="type" label="类型" initialValue={2}>
            <Segmented
              block
              options={[
                { label: '目录', value: 1 },
                { label: '菜单', value: 2 },
                { label: '按钮', value: 3 },
              ]}
            />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入菜单名称' }]}>
            <Input placeholder="例如：用户管理" />
          </Form.Item>
          <Form.Item name="sort" label="排序" initialValue={0}>
            <InputNumber min={0} max={9999} style={{ width: '100%' }} />
          </Form.Item>
          {(type === undefined || type !== 3) && (
            <>
              <Form.Item name="path" label="路由路径">
                <Input placeholder="/system/users" />
              </Form.Item>
              <Form.Item name="component" label="前端组件">
                <Input placeholder="system/users" />
              </Form.Item>
            </>
          )}
          {type !== 1 && (
            <Form.Item name="permission" label="权限码" className={type === 3 ? 'full' : undefined}>
              <Input placeholder="system:user:create" />
            </Form.Item>
          )}
          <Form.Item name="icon" label="图标">
            <Input placeholder="users / shield / folder" />
          </Form.Item>
          <Form.Item name="visible" label="侧栏可见" valuePropName="checked" initialValue>
            <Switch />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '正常', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
        </div>
      </FormModal>
    </>
  )
}
