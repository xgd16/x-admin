/**
 * 用户管理：列表、检查器编辑、导入导出与密码重置。
 */
import { useEffect, useRef, useState, type Key } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Drawer, Form, Input, Modal, Select } from 'antd'
import { useSearchParams } from 'react-router-dom'
import { api, saveBlob } from '../api/client'
import type { Department, ImportResult, PageResult, Role, User } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { DataTable } from '../components/DataTable'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'
import { StatusTag } from '../components/StatusTag'
import { useIsCompactViewport } from '../hooks/useMediaQuery'
import { useToast } from '../hooks/useToast'
import { flattenTree, formatDate, initials } from '../utils/format'

/** 新建用户表单字段。 */
interface UserFormValues {
  username: string
  password?: string
  nickname: string
  email: string
  phone: string
  dept_id?: number
  status: 0 | 1
  role_ids?: number[]
  remark?: string
}

/** 右侧检查器可编辑的字段。 */
interface InspectorValues {
  nickname: string
  email: string
  phone: string
  dept_id?: number
  status: 0 | 1
}

export function Users() {
  const { modal } = App.useApp()
  const toast = useToast()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const isMobile = useIsCompactViewport()
  const [params, setParams] = useSearchParams()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [keywordDraft, setKeywordDraft] = useState('')
  const [deptID, setDeptID] = useState<number | undefined>()
  const [status, setStatus] = useState<number | undefined>()
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([])
  const [selectedID, setSelectedID] = useState<number>()
  const [inspectorOpen, setInspectorOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [resetUser, setResetUser] = useState<User>()
  const [roleIDs, setRoleIDs] = useState<number[]>([])
  const [createForm] = Form.useForm<UserFormValues>()
  const [resetForm] = Form.useForm<{ password: string }>()
  const [inspectorForm] = Form.useForm<InspectorValues>()
  const fileInput = useRef<HTMLInputElement>(null)

  // 用户列表按分页、搜索词、部门与状态组合查询。
  const usersQuery = useQuery({
    queryKey: ['users', page, pageSize, keyword, deptID, status],
    queryFn: () =>
      api.get<PageResult<User>>('/users', {
        page,
        page_size: pageSize,
        keyword,
        dept_id: deptID,
        status,
      }),
  })

  // 部门与角色是筛选/表单的选项数据，只在有对应权限时加载。
  const deptsQuery = useQuery({
    queryKey: ['departments-tree'],
    queryFn: () => api.get<Department[]>('/departments/tree'),
    enabled: can('system:dept:list'),
  })

  const rolesQuery = useQuery({
    queryKey: ['roles-all'],
    queryFn: () => api.get<Role[]>('/roles/all'),
    enabled: can('system:role:list'),
    staleTime: 60_000,
  })

  const list = usersQuery.data?.list ?? []
  const selected = list.find((user) => user.id === selectedID) ?? list[0]
  const depts = flattenTree(deptsQuery.data ?? [])
  const roles = rolesQuery.data ?? []

  // 保证右侧检查器始终有选中用户；列表变化后自动纠正失效的 id。
  useEffect(() => {
    if (!selectedID && list.length > 0) setSelectedID(list[0].id)
    if (selectedID && list.length > 0 && !list.some((user) => user.id === selectedID)) setSelectedID(list[0].id)
  }, [list, selectedID])

  // 切换选中用户时，把资料与角色回填到检查器表单。
  // 手机上检查器放在抽屉里，未打开时不写表单，避免未挂载实例告警。
  useEffect(() => {
    if (selected && (!isMobile || inspectorOpen)) {
      inspectorForm.setFieldsValue({
        nickname: selected.nickname,
        email: selected.email,
        phone: selected.phone,
        dept_id: selected.dept_id ?? undefined,
        status: selected.status,
      })
      setRoleIDs(selected.roles?.map((role) => role.id) ?? [])
    }
  }, [selected?.id, inspectorForm, selected, isMobile, inspectorOpen])

  // 支持从仪表盘通过 ?new=1 直接打开“新建用户”弹窗。
  useEffect(() => {
    if (params.get('new') === '1' && can('system:user:create')) {
      setCreateOpen(true)
      params.delete('new')
      setParams(params, { replace: true })
    }
  }, [params, setParams, can])

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['users'] })

  // —— 用户增删改、密码重置与批量导入 ——
  const createMutation = useMutation({
    mutationFn: (values: UserFormValues) => api.post<User>('/users', values),
    onSuccess: () => {
      toast.success('用户已创建')
      setCreateOpen(false)
      createForm.resetFields()
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const updateMutation = useMutation({
    mutationFn: (values: InspectorValues & { role_ids: number[] }) =>
      api.put<User>(`/users/${selected?.id}`, values),
    onSuccess: (user) => {
      toast.success('变更已保存')
      inspectorForm.setFieldsValue({ ...user, dept_id: user.dept_id ?? undefined })
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (ids: number[]) => api.del('/users', { ids }),
    onSuccess: () => {
      toast.success('已删除')
      setSelectedKeys([])
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const resetMutation = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) => api.put(`/users/${id}/password`, { password }),
    onSuccess: () => {
      toast.success('密码已重置')
      setResetUser(undefined)
      resetForm.resetFields()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const importMutation = useMutation({
    mutationFn: (file: File) => api.upload<ImportResult>('/users/import', file),
    onSuccess: (result) => {
      if (result.failed.length > 0) {
        // 导入失败行较多时用通知卡片展开，避免轻提示只显示第一行。
        toast.notify({
          type: 'warning',
          title: `导入完成：成功 ${result.success} 个，失败 ${result.failed.length} 个`,
          key: 'user-import-result',
          description: (
            <div className="toast-detail">
              {result.failed.slice(0, 5).map((row) => (
                <p key={`${row.row}-${row.username}`}>
                  第 {row.row} 行 · <b>{row.username || '未命名'}</b>：{row.message}
                </p>
              ))}
              {result.failed.length > 5 && <p>另有 {result.failed.length - 5} 行失败，请修正后重新导入。</p>}
            </div>
          ),
          duration: 8,
        })
      } else if (result.success > 0) {
        toast.success(`成功导入 ${result.success} 个用户`)
      } else {
        toast.info('没有可导入的数据')
      }
      void invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  // 导出改为提交异步任务：后台分页生成 xlsx，完成后在导出记录页下载。
  const exportMutation = useMutation({
    mutationFn: () =>
      api.post('/export-tasks', {
        module: 'users',
        params: { keyword, dept_id: deptID, status },
      }),
    onSuccess: () => {
      toast.success('已加入导出队列，完成后可在「导出记录」页下载')
      void queryClient.invalidateQueries({ queryKey: ['export-tasks'] })
    },
    onError: (error: Error) => toast.fail(error),
  })

  const downloadTemplate = () => {
    void toast
      .promise(api.download('/users/template').then((blob) => saveBlob(blob, 'user-import-template.xlsx')), {
        loading: '正在下载模板…',
        success: '导入模板已下载',
        error: (error) => (error instanceof Error ? error.message : '下载失败'),
      })
      .catch(() => undefined)
  }

  // 删除前二次确认；历史操作日志不会被一并删除。
  const confirmDelete = (ids: number[]) => {
    if (ids.length === 0) {
      toast.warning('请先选择用户')
      return
    }
    modal.confirm({
      title: `删除 ${ids.length} 个用户？`,
      content: '用户将从列表移除，但其历史日志会保留。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => deleteMutation.mutateAsync(ids),
    })
  }

  const toggleRole = (id: number) => {
    setRoleIDs((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]))
  }

  // 桌面显示右侧检查器，手机复用同一份内容放进抽屉。
  const inspectorContent = selected ? (
    <>
      <div className="ins-head">
        <div className="big-avatar">{initials(selected.nickname || selected.username)}</div>
        <div>
          <b>{selected.nickname || selected.username}</b>
          <span>
            {selected.email || '未设置邮箱'} · UID {selected.id}
          </span>
          <span>最后登录 {formatDate(selected.last_login_at)}</span>
        </div>
      </div>
      <Form form={inspectorForm} layout="vertical" onFinish={(values) => updateMutation.mutate({ ...values, role_ids: roleIDs })}>
        <div className="ins-body">
          <div className="field">
            <label>显示名称</label>
            <Form.Item name="nickname" noStyle>
              <Input placeholder="显示名称" />
            </Form.Item>
          </div>
          <div className="field">
            <label>邮箱</label>
            <Form.Item name="email" noStyle>
              <Input placeholder="name@example.com" />
            </Form.Item>
          </div>
          <div className="field">
            <label>手机号</label>
            <Form.Item name="phone" noStyle>
              <Input placeholder="13800000000" />
            </Form.Item>
          </div>
          <div className="field">
            <label>所属部门</label>
            <Form.Item name="dept_id" noStyle>
              <Select allowClear options={depts.map((dept) => ({ label: dept.name, value: dept.id }))} placeholder="选择部门" />
            </Form.Item>
          </div>
          <div className="field">
            <label>角色</label>
            <div className="role-chips">
              {roles.map((role) => (
                <label className="role-chip" key={role.id}>
                  <input type="checkbox" checked={roleIDs.includes(role.id)} onChange={() => toggleRole(role.id)} />
                  <span>{role.name}</span>
                </label>
              ))}
            </div>
          </div>
          <div className="field">
            <label>账号状态</label>
            <Form.Item name="status" noStyle>
              <Select
                options={[
                  { label: '正常', value: 1 },
                  { label: '停用', value: 0 },
                ]}
              />
            </Form.Item>
          </div>
        </div>
        <div className="ins-foot">
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <span className="hint">变更写入操作日志</span>
            {can('system:user:reset') && (
              <Button size="small" type="text" onClick={() => setResetUser(selected)}>
                重置密码
              </Button>
            )}
          </div>
          <Button type="primary" htmlType="submit" loading={updateMutation.isPending}>
            保存变更
          </Button>
        </div>
      </Form>
    </>
  ) : (
    <div className="empty-hint">选择一行查看用户详情</div>
  )

  return (
    <>
      <PageHeader
        title="用户管理"
        sub={
          <>
            {usersQuery.data?.total ?? 0} 个账号 · 已选 {selectedKeys.length} 项 · 最后同步 {formatDate(new Date().toISOString())}
          </>
        }
        actions={
          <>
            {can('system:user:import') && (
              <button className="btn" type="button" onClick={() => fileInput.current?.click()}>
                <Icon name="upload" size={13} />
                导入
              </button>
            )}
            {can('system:user:create') && (
              <button className="btn primary" type="button" onClick={() => setCreateOpen(true)}>
                <Icon name="plus" size={13} />
                新建用户
              </button>
            )}
          </>
        }
      />

      {/* 左侧列表 + 右侧检查器：点击行即可快速编辑，不必打开弹窗。 */}
      <div className="users-layout">
        <DataTable<User>
          storageKey="users"
          rowKey="id"
          columns={[
            {
              title: '用户',
              key: 'user',
              width: 200,
              render: (_, row) => (
                <div className="cell-user">
                  <span className="cell-avatar">{initials(row.nickname || row.username)}</span>
                  <span>
                    <b>{row.nickname || row.username}</b>
                    <small>{row.email || row.username}</small>
                  </span>
                </div>
              ),
            },
            { title: '部门', key: 'dept', width: 90, render: (_, row) => row.dept?.name ?? '—' },
            {
              title: '角色',
              key: 'roles',
              width: 150,
              render: (_, row) => (
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {(row.roles ?? []).map((role) => (
                    <span key={role.id} className="tag" style={{ display: 'inline-flex', padding: '1px 6px', borderRadius: 999, border: '1px solid var(--line)', fontSize: 8.5 }}>
                      {role.name}
                    </span>
                  ))}
                  {(row.roles ?? []).length === 0 && <span className="mono">未分配</span>}
                </div>
              ),
            },
            { title: '状态', key: 'status', width: 80, render: (_, row) => <StatusTag status={row.status} /> },
            {
              title: '操作',
              key: 'actions',
              width: 80,
              fixed: 'right',
              render: (_, row) => (
                <Button
                  size="small"
                  type="text"
                  onClick={(event) => {
                    event.stopPropagation()
                    setSelectedID(row.id)
                    if (isMobile) setInspectorOpen(true)
                  }}
                >
                  编辑
                </Button>
              ),
            },
          ]}
          data={list}
          loading={usersQuery.isFetching}
          total={usersQuery.data?.total ?? 0}
          page={page}
          pageSize={pageSize}
          onPageChange={(nextPage, nextSize) => {
            setPage(nextPage)
            setPageSize(nextSize)
          }}
          onRefresh={() => void usersQuery.refetch()}
          onRow={(row) => ({
            onClick: () => {
              setSelectedID(row.id)
              if (isMobile) setInspectorOpen(true)
            },
            style: { cursor: 'pointer' },
          })}
          filters={
            <>
              <Input
                allowClear
                autoComplete="off"
                placeholder="搜索姓名 / 邮箱 / 手机号"
                value={keywordDraft}
                onChange={(event) => setKeywordDraft(event.target.value)}
                onPressEnter={() => {
                  setKeyword(keywordDraft)
                  setPage(1)
                }}
                style={{ minWidth: 220, flex: 1 }}
              />
              {can('system:dept:list') && (
                <Select
                  allowClear
                  placeholder="全部部门"
                  value={deptID}
                  options={depts.map((dept) => ({ label: dept.name, value: dept.id }))}
                  onChange={(value) => {
                    setDeptID(value)
                    setPage(1)
                  }}
                  style={{ width: 128 }}
                />
              )}
              <Select
                allowClear
                placeholder="全部状态"
                value={status}
                options={[
                  { label: '正常', value: 1 },
                  { label: '停用', value: 0 },
                ]}
                onChange={(value) => {
                  setStatus(value)
                  setPage(1)
                }}
                style={{ width: 108 }}
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
            <>
              {can('system:user:create') && (
                <Button type="primary" onClick={() => setCreateOpen(true)}>
                  <Icon name="plus" size={12} /> 新建
                </Button>
              )}
              {can('system:user:import') && (
                <Button onClick={() => fileInput.current?.click()}>
                  <Icon name="upload" size={12} /> 导入
                </Button>
              )}
              {can('system:user:export') && (
                <Button loading={exportMutation.isPending} onClick={() => exportMutation.mutate()}>
                  <Icon name="download" size={12} /> 导出
                </Button>
              )}
              {can('system:user:delete') && (
                <Button danger onClick={() => confirmDelete(selectedKeys.map(Number))}>
                  <Icon name="trash" size={12} /> 删除
                </Button>
              )}
              <Button type="text" onClick={() => void downloadTemplate()}>
                模板
              </Button>
              <input
                ref={fileInput}
                type="file"
                accept=".xlsx,.xls"
                hidden
                onChange={(event) => {
                  const file = event.target.files?.[0]
                  if (file) importMutation.mutate(file)
                  event.target.value = ''
                }}
              />
            </>
          }
          selection={{
            selectedRowKeys: selectedKeys,
            onChange: (keys) => setSelectedKeys(keys),
          }}
        />

        {!isMobile && <aside className="inspector glass">{inspectorContent}</aside>}
      </div>

      {/* 手机上检查器改为右侧抽屉，避免编辑区被长列表推到页面底部。 */}
      {isMobile && inspectorOpen && selected && (
        <Drawer
          open
          placement="right"
          size="min(420px, 100vw)"
          title={`用户详情 · ${selected.nickname || selected.username}`}
          onClose={() => setInspectorOpen(false)}
          destroyOnHidden
        >
          <div className="inspector-drawer">{inspectorContent}</div>
        </Drawer>
      )}

      {/* 新建用户弹窗 */}
      <FormModal
        open={createOpen}
        title="新建用户"
        form={createForm}
        onCancel={() => setCreateOpen(false)}
        onSubmit={(values) => createMutation.mutate(values as unknown as UserFormValues)}
        confirmLoading={createMutation.isPending}
      >
        <div className="detail-grid">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input autoComplete="username" placeholder="3-32 位字母、数字或下划线" />
          </Form.Item>
          <Form.Item name="password" label="初始密码" rules={[{ required: true, message: '请输入初始密码' }]}>
            <Input.Password autoComplete="new-password" placeholder="至少 8 位，含字母和数字" />
          </Form.Item>
          <Form.Item name="nickname" label="昵称">
            <Input autoComplete="nickname" placeholder="显示名称" />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <Select options={[{ label: '正常', value: 1 }, { label: '停用', value: 0 }]} />
          </Form.Item>
          <Form.Item name="email" label="邮箱">
            <Input autoComplete="email" placeholder="name@example.com" />
          </Form.Item>
          <Form.Item name="phone" label="手机号">
            <Input autoComplete="tel" placeholder="13800000000" />
          </Form.Item>
          <Form.Item name="dept_id" label="部门">
            <Select allowClear options={depts.map((dept) => ({ label: dept.name, value: dept.id }))} />
          </Form.Item>
          <Form.Item name="role_ids" label="角色">
            <Select mode="multiple" options={roles.map((role) => ({ label: role.name, value: role.id }))} />
          </Form.Item>
          <Form.Item name="remark" label="备注" className="full">
            <Input autoComplete="off" placeholder="可选" />
          </Form.Item>
        </div>
      </FormModal>

      {/* 管理员重置密码弹窗 */}
      <Modal
        open={!!resetUser}
        title={`重置密码 · ${resetUser?.nickname ?? ''}`}
        okText="重置"
        cancelText="取消"
        confirmLoading={resetMutation.isPending}
        onCancel={() => setResetUser(undefined)}
        onOk={() => resetForm.submit()}
        destroyOnHidden
      >
        <Form form={resetForm} layout="vertical" onFinish={(values) => resetUser && resetMutation.mutate({ id: resetUser.id, password: values.password })}>
          <Form.Item name="password" label="新密码" rules={[{ required: true, message: '请输入新密码' }]}>
            <Input.Password autoComplete="new-password" placeholder="至少 8 位，含字母和数字" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}
