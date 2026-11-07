/**
 * 菜单管理：目录/菜单/按钮三级节点维护，桌面端支持拖拽调整层级与顺序。
 */
import { useEffect, useMemo, useRef, useState, type Key } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  App,
  Button,
  Dropdown,
  Empty,
  Form,
  Input,
  InputNumber,
  Segmented,
  Select,
  Switch,
  Tree,
  TreeSelect,
} from 'antd'
import type { TreeDataNode, TreeProps } from 'antd'
import { api } from '../api/client'
import type { Menu } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { FormModal } from '../components/FormModal'
import { Icon } from '../components/Icon'
import { TreeSkeleton } from '../components/Loading'
import { StatusTag } from '../components/StatusTag'
import { useIsCompactViewport } from '../hooks/useMediaQuery'
import { useToast } from '../hooks/useToast'
import { flattenTree } from '../utils/format'
import { menuContainsId, moveMenuNode, type MenuOrderUpdate } from '../utils/menuTree'

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

interface MenuTreeNode extends TreeDataNode {
  key: number
  menu: Menu
  children?: MenuTreeNode[]
}

/** 把菜单树转换成 antd Tree 需要的结构，业务字段挂在 menu 上。 */
function toTreeData(menus: Menu[]): MenuTreeNode[] {
  return menus.map((menu) => ({
    key: menu.id,
    title: menu.name,
    menu,
    children: menu.children?.length ? toTreeData(menu.children) : undefined,
  }))
}

/** 把菜单树转换成 TreeSelect 需要的结构。 */
function toTreeSelect(menus: Menu[]): Array<{ title: string; value: number; children?: ReturnType<typeof toTreeSelect> }> {
  return menus.map((menu) => ({
    title: menu.name,
    value: menu.id,
    children: menu.children ? toTreeSelect(menu.children) : undefined,
  }))
}

// 菜单节点类型：1 目录、2 菜单、3 按钮。
const typeLabels: Record<number, string> = { 1: '目录', 2: '菜单', 3: '按钮' }

function typeTagClass(type: number): string {
  return type === 3 ? 'orange' : type === 2 ? 'green' : ''
}

export function Menus() {
  const { modal } = App.useApp()
  const toast = useToast()
  const queryClient = useQueryClient()
  const can = useAuth((state) => state.can)
  const compact = useIsCompactViewport()
  const canCreate = can('system:menu:create')
  const canUpdate = can('system:menu:update')
  const canDelete = can('system:menu:delete')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Menu>()
  const [expanded, setExpanded] = useState<Key[]>([])
  const [form] = Form.useForm<MenuValues>()
  const type = Form.useWatch('type', form)

  // 菜单树一次加载，前端负责展开、父级选择和拖拽排序。
  const menusQuery = useQuery({
    queryKey: ['menus-tree'],
    queryFn: () => api.get<Menu[]>('/menus/tree'),
  })

  const tree = useMemo(() => menusQuery.data ?? [], [menusQuery.data])
  const treeData = useMemo(() => toTreeData(tree), [tree])
  const flat = useMemo(() => flattenTree(tree), [tree])
  const allKeys = useMemo(() => flat.map((menu) => menu.id), [flat])

  // 首次加载时桌面全部展开，手机只保留根节点，避免一屏塞进几十个节点。
  const expandedInit = useRef(false)
  const lastCompact = useRef(compact)
  useEffect(() => {
    if (!menusQuery.data) return
    if (expandedInit.current && lastCompact.current === compact) return
    expandedInit.current = true
    lastCompact.current = compact
    setExpanded(compact ? [] : allKeys)
  }, [menusQuery.data, compact, allKeys])

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['menus-tree'] })
    void queryClient.invalidateQueries({ queryKey: ['menus'] })
  }

  // 菜单变更后同时刷新树和扁平列表，保证其它页面拿到最新权限数据。
  const saveMutation = useMutation({
    mutationFn: (values: MenuValues) =>
      editing ? api.put<Menu>(`/menus/${editing.id}`, values) : api.post<Menu>('/menus', values),
    onSuccess: () => {
      toast.success(editing ? '菜单已更新' : '菜单已创建')
      setOpen(false)
      setEditing(undefined)
      form.resetFields()
      invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.del(`/menus/${id}`),
    onSuccess: () => {
      toast.success('菜单已删除')
      invalidate()
    },
    onError: (error: Error) => toast.fail(error),
  })

  // 拖拽后批量写回受影响的节点；后端 Update 接口接收完整菜单字段。
  const reorderMutation = useMutation({
    mutationFn: async (updates: MenuOrderUpdate[]) => {
      await Promise.all(
        updates.map(({ menu, parentId, sort }) =>
          api.put<Menu>(`/menus/${menu.id}`, {
            parent_id: parentId,
            name: menu.name,
            type: menu.type,
            path: menu.path,
            component: menu.component,
            permission: menu.permission,
            icon: menu.icon,
            sort,
            visible: menu.visible,
            status: menu.status,
          }),
        ),
      )
    },
    onSuccess: () => {
      toast.success('菜单层级与顺序已更新')
      invalidate()
    },
    onError: (error: Error) => {
      toast.fail(error)
      // 批量更新可能只成功一部分，失败后重新拉取服务端状态。
      invalidate()
    },
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

  const allExpanded = allKeys.length > 0 && allKeys.every((key) => expanded.includes(key))

  const toggleExpandAll = () => {
    setExpanded(allExpanded ? [] : allKeys)
  }

  const allowDrop: NonNullable<TreeProps<MenuTreeNode>['allowDrop']> = ({ dragNode, dropNode, dropPosition }) => {
    if (!canUpdate) return false
    if (dragNode.menu.id === dropNode.menu.id) return false
    // 按钮节点没有子级，不能作为容器。
    if (dropPosition === 0 && dropNode.menu.type === 3) return false
    // 不能把父节点拖进自己的子树。
    return !menuContainsId(dragNode.menu, dropNode.menu.id)
  }

  const handleDrop: NonNullable<TreeProps<MenuTreeNode>['onDrop']> = (info) => {
    if (!canUpdate || reorderMutation.isPending) return
    const dropId = Number(info.node.key)
    const dropToGap = Boolean(info.dropToGap)
    const result = moveMenuNode(
      tree,
      Number(info.dragNode.key),
      dropId,
      info.dropPosition,
      dropToGap,
    )
    if (!result) {
      toast.warning('不能移动到自己的子级或按钮节点下')
      return
    }
    if (result.updates.length === 0) return
    // 拖进折叠节点后自动展开，避免节点从视图中"消失"。
    if (!dropToGap) {
      setExpanded((keys) => (keys.includes(dropId) ? keys : [...keys, dropId]))
    }
    // 先乐观更新界面，再批量写回后端。
    queryClient.setQueryData(['menus-tree'], result.tree)
    reorderMutation.mutate(result.updates)
  }

  const renderMenuTitle = (node: MenuTreeNode) => {
    const menu = node.menu
    const actionItems = [
      canUpdate ? { key: 'edit', label: '编辑' } : null,
      canDelete ? { key: 'delete', label: '删除', danger: true } : null,
    ].filter((item): item is { key: string; label: string; danger?: boolean } => item !== null)

    return (
      <div className="menu-tree-node">
        <span className="menu-tree-icon">
          <Icon name={menu.icon || 'file'} size={13} />
        </span>
        <div className="menu-tree-main">
          <b className="menu-tree-name" title={menu.name}>
            {menu.name}
          </b>
          <span className={`tag menu-tree-type ${typeTagClass(menu.type)}`}>{typeLabels[menu.type]}</span>
          {!menu.visible && <span className="tag menu-tree-type">隐藏</span>}
          <StatusTag status={menu.status} />
        </div>
        {(menu.path || menu.permission) && (
          <div className="menu-tree-meta">
            {menu.path && (
              <span className="menu-tree-code" title={menu.path}>
                {menu.path}
              </span>
            )}
            {menu.permission && (
              <span className="menu-tree-permission" title={menu.permission}>
                {menu.permission}
              </span>
            )}
          </div>
        )}
        {compact ? (
          actionItems.length > 0 && (
            <Dropdown
              trigger={['click']}
              menu={{
                items: actionItems,
                onClick: ({ key, domEvent }) => {
                  domEvent.stopPropagation()
                  if (key === 'edit') openEdit(menu)
                  else confirmDelete(menu)
                },
              }}
            >
              <button
                type="button"
                className="icon-btn menu-tree-more"
                title="更多操作"
                aria-label={`更多操作：${menu.name}`}
                onClick={(event) => event.stopPropagation()}
              >
                <Icon name="more" size={15} />
              </button>
            </Dropdown>
          )
        ) : (
          actionItems.length > 0 && (
            <div className="menu-tree-actions" onClick={(event) => event.stopPropagation()}>
              {canUpdate && (
                <Button size="small" type="text" onClick={() => openEdit(menu)}>
                  编辑
                </Button>
              )}
              {canDelete && (
                <Button size="small" type="text" danger onClick={() => confirmDelete(menu)}>
                  删除
                </Button>
              )}
            </div>
          )
        )}
      </div>
    )
  }

  return (
    <>
      <PageHeader
        title="菜单管理"
        sub={`${flat.length} 个节点 · 目录 / 菜单 / 按钮三级权限 · 修改后重新登录生效`}
        actions={
          canCreate ? (
            <button className="btn primary" type="button" onClick={openCreate}>
              <Icon name="plus" size={13} /> 新建菜单
            </button>
          ) : undefined
        }
      />

      <div
        className="data-panel glass menu-tree-panel"
        aria-busy={menusQuery.isFetching || reorderMutation.isPending || undefined}
      >
        <div className="table-toolbar menu-tree-toolbar">
          <div className="left">
            <span className="menu-tree-hint">
              {compact ? '通过「编辑」调整上级与排序' : '拖拽节点可调整层级与顺序'}
            </span>
          </div>
          <div className="right">
            {allKeys.length > 0 && (
              <button
                className="icon-btn"
                type="button"
                title={allExpanded ? '全部收起' : '全部展开'}
                aria-label={allExpanded ? '全部收起' : '全部展开'}
                onClick={toggleExpandAll}
              >
                <Icon name={allExpanded ? 'chevronRight' : 'chevronDown'} size={15} />
              </button>
            )}
            <button
              className="icon-btn"
              type="button"
              title="刷新"
              aria-busy={menusQuery.isFetching || reorderMutation.isPending || undefined}
              onClick={() => void menusQuery.refetch()}
            >
              <Icon
                name="refresh"
                size={15}
                className={menusQuery.isFetching || reorderMutation.isPending ? 'x-spin-icon' : undefined}
              />
            </button>
          </div>
        </div>
        <div className="menu-tree-body">
          {menusQuery.isLoading ? (
            <TreeSkeleton rows={7} />
          ) : menusQuery.isError ? (
            <div className="menu-tree-empty">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="菜单加载失败" />
              <Button size="small" onClick={() => void menusQuery.refetch()}>
                重试
              </Button>
            </div>
          ) : treeData.length === 0 ? (
            <div className="menu-tree-empty">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有菜单节点" />
              {canCreate && (
                <Button type="primary" onClick={openCreate}>
                  <Icon name="plus" size={12} /> 新建菜单
                </Button>
              )}
            </div>
          ) : (
            <Tree<MenuTreeNode>
              className="menu-tree"
              blockNode
              treeData={treeData}
              titleRender={renderMenuTitle}
              selectable={false}
              showLine={!compact}
              switcherIcon={({ expanded, isLeaf }) =>
                isLeaf ? null : (
                  <Icon
                    name="chevronDown"
                    size={14}
                    className={`menu-tree-switcher-icon${expanded ? ' is-open' : ''}`}
                  />
                )
              }
              expandedKeys={expanded}
              onExpand={(keys) => setExpanded(keys)}
              autoExpandParent={false}
              draggable={canUpdate && !reorderMutation.isPending && !menusQuery.isFetching}
              allowDrop={allowDrop}
              onDrop={handleDrop}
            />
          )}
        </div>
      </div>

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
          {/* 按钮节点没有路由；目录节点没有权限码。 */}
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
