/**
 * 角色管理与菜单权限分配。
 */
import { useEffect, useState, type Key } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, InputNumber, Modal, Select, Tree } from 'antd'
import type { TreeDataNode } from 'antd'
import { api } from '../api/client'
import type { Menu, PageResult, Role } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'
import { StatusTag } from '../components/StatusTag'

interface RoleValues {
  name: string
  code: string
  status: 0 | 1
  sort: number
  remark: string
}

/** 把菜单树转换成 antd Tree 需要的节点结构。 */
function toTreeData(menus: Menu[]): TreeDataNode[] {
  return menus.map((menu) => ({
    key: menu.id,
    title: menu.name,
    children: menu.children ? toTreeData(menu.children) : undefined,
  }))
}

export function Roles() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<Role>()
  const [assigning, setAssigning] = useState<Role>()
  const [checkedKeys, setCheckedKeys] = useState<Key[]>([])
  const [halfChecked, setHalfChecked] = useState<Key[]>([])
  const [form] = Form.useForm<RoleValues>()

  // 角色列表用于表格，all 列表用于其它页面的下拉选项。
  const rolesQuery = useQuery({
    queryKey: ['roles', page, pageSize, keyword],
    queryFn: () => api.get<PageResult<Role>>('/roles', { page, page_size: pageSize, keyword }),
  })

  const allQuery = useQuery({
    queryKey: ['roles-all'],
    queryFn: () => api.get<Role[]>('/roles/all'),
    staleTime: 60_000,
  })

  const menuQuery = useQuery({
    queryKey: ['menus-tree'],
    queryFn: () => api.get<Menu[]>('/menus/tree'),
    enabled: !!assigning,
    staleTime: 60_000,
  })

  const roleMenusQuery = useQuery({
    queryKey: ['role-menus', assigning?.id],
    queryFn: () => api.get<number[]>(`/roles/${assigning?.id}/menus`),
    enabled: !!assigning,
  })

  // 打开权限弹窗后，把角色已有菜单回填到树。
  useEffect(() => {
    if (roleMenusQuery.data) {
      setCheckedKeys(roleMenusQuery.data)
      setHalfChecked([])
    }
  }, [roleMenusQuery.data])

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['roles'] })
    void queryClient.invalidateQueries({ queryKey: ['roles-all'] })
  }

  // —— 角色 CRUD 与菜单权限分配 ——
  const saveMutation = useMutation({
    mutationFn: (values: RoleValues) =>
      editing ? api.put<Role>(`/roles/${editing.id}`, values) : api.post<Role>('/roles', values),
    onSuccess: () => {
      message.success(editing ? '角色已更新' : '角色已创建')
      setCreateOpen(false)
      setEditing(undefined)
      form.resetFields()
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/roles/${id}`),
    onSuccess: () => {
      message.success('角色已删除')
      invalidate()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const assignMutation = useMutation({
    mutationFn: (menuIDs: number[]) => api.put(`/roles/${assigning?.id}/menus`, { menu_ids: menuIDs }),
    onSuccess: () => {
      message.success('权限已保存')
      setAssigning(undefined)
      void queryClient.invalidateQueries({ queryKey: ['role-menus'] })
    },
    onError: (error: Error) => message.error(error.message),
  })

  const openCreate = () => {
    setEditing(undefined)
    form.resetFields()
    form.setFieldsValue({ status: 1, sort: 0, remark: '' })
    setCreateOpen(true)
  }

  const openEdit = (role: Role) => {
    setEditing(role)
    form.setFieldsValue(role)
    setCreateOpen(true)
  }

  const confirmDelete = (role: Role) => {
    modal.confirm({
      title: `删除角色「${role.name}」？`,
      content: '内置超级管理员不可删除；已分配用户的角色也无法删除。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => deleteMutation.mutateAsync(role.id),
    })
  }

  return (
    <>
      <PageHeader
        title="角色管理"
        sub={`${rolesQuery.data?.total ?? 0} 个角色 · 按权限码控制菜单与按钮 · 全量 ${allQuery.data?.length ?? 0} 个可用角色`}
        actions={
          can('system:role:create') ? (
            <button className="btn primary" type="button" onClick={openCreate}>
              <Icon name="plus" size={13} /> 新建角色
            </button>
          ) : undefined
        }
      />

      <DataTable<Role>
        storageKey="roles"
        rowKey="id"
        columns={[
          { title: '角色名称', dataIndex: 'name', key: 'name', width: 170, render: (value: string) => <b style={{ fontSize: 11 }}>{value}</b> },
          { title: '编码', dataIndex: 'code', key: 'code', width: 150, render: (value: string) => <span className="mono">{value}</span> },
          { title: '排序', dataIndex: 'sort', key: 'sort', width: 70 },
          { title: '状态', dataIndex: 'status', key: 'status', width: 90, render: (value: 0 | 1) => <StatusTag status={value} /> },
          { title: '备注', dataIndex: 'remark', key: 'remark', render: (value: string) => value || '—' },
          {
            title: '操作',
            key: 'actions',
            width: 200,
            fixed: 'right',
            render: (_, role) => (
              <div className="inline-actions">
                {can('system:role:assign') && (
                  <Button
                    size="small"
                    type="text"
                    onClick={() => {
                      setAssigning(role)
                      setCheckedKeys([])
                      setHalfChecked([])
                    }}
                  >
                    分配权限
                  </Button>
                )}
                {can('system:role:update') && (
                  <Button size="small" type="text" onClick={() => openEdit(role)}>
                    编辑
                  </Button>
                )}
                {can('system:role:delete') && (
                  <Button size="small" type="text" danger onClick={() => confirmDelete(role)}>
                    删除
                  </Button>
                )}
              </div>
            ),
          },
        ]}
        data={rolesQuery.data?.list ?? []}
        loading={rolesQuery.isFetching}
        total={rolesQuery.data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={(nextPage, nextSize) => {
          setPage(nextPage)
          setPageSize(nextSize)
        }}
        onRefresh={() => void rolesQuery.refetch()}
        filters={
          <>
            <Input
              allowClear
              placeholder="搜索角色名称 / 编码"
              value={keywordDraft}
              onChange={(event) => setKeywordDraft(event.target.value)}
              onPressEnter={() => {
                setKeyword(keywordDraft)
                setPage(1)
              }}
              style={{ minWidth: 220, flex: 1 }}
            />
            <Button
              onClick={() => {
                setKeyword(keywordDraft)
                setPage(1)
              }}
            >
              <Icon name="search" size={12} /> 筛选
            </Button>
          </>
        }
        toolbar={
          can('system:role:create') ? (
            <Button type="primary" onClick={openCreate}>
              <Icon name="plus" size={12} /> 新建
            </Button>
          ) : undefined
        }
      />

      <FormModal
        open={createOpen}
        title={editing ? '编辑角色' : '新建角色'}
        form={form}
        onCancel={() => {
          setCreateOpen(false)
          setEditing(undefined)
        }}
        onSubmit={(values) => saveMutation.mutate(values as unknown as RoleValues)}
        confirmLoading={saveMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input placeholder="例如：运营管理员" />
          </Form.Item>
          <Form.Item name="code" label="角色编码" rules={[{ required: true, message: '请输入角色编码' }]}>
            <Input placeholder="例如：operator" disabled={editing?.code === 'super_admin'} />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '正常', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
          <Form.Item name="sort" label="排序" initialValue={0}>
            <InputNumber min={0} max={9999} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注" className="full">
            <Input placeholder="可选" />
          </Form.Item>
        </div>
      </FormModal>

      {/* 半选父节点也要提交，后端据此构建完整菜单树。 */}
      <Modal
        open={!!assigning}
        title={`分配权限 · ${assigning?.name ?? ''}`}
        width={520}
        okText="保存"
        cancelText="取消"
        confirmLoading={assignMutation.isPending}
        onCancel={() => setAssigning(undefined)}
        onOk={() => assignMutation.mutate(Array.from(new Set([...checkedKeys, ...halfChecked].map(Number))))}
        destroyOnHidden
      >
        {menuQuery.isLoading ? (
          <div className="empty-hint">加载菜单中…</div>
        ) : (
          <Tree
            checkable
            defaultExpandAll
            treeData={toTreeData(menuQuery.data ?? [])}
            checkedKeys={checkedKeys}
            onCheck={(keys, info) => {
              setCheckedKeys(Array.isArray(keys) ? keys : keys.checked)
              setHalfChecked(info.halfCheckedKeys ?? [])
            }}
            style={{ maxHeight: 420, overflow: 'auto' }}
          />
        )}
      </Modal>
    </>
  )
}
