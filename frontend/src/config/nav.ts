/**
 * 侧边导航、模块分组、快捷操作与路由匹配配置。
 *
 * navItems / railGroups 是前端已知路由的元数据；运行时以 /auth/me 返回的
 * 菜单树为准（见 useNav），这样管理员调整菜单名称、图标、排序或角色授权后
 * 无需重新构建前端。
 */
import { useMemo } from 'react'
import type { Menu } from '../api/types'
import { useAuth } from '../stores/auth'

export interface NavItem {
  key: string
  label: string
  path: string
  icon: string
  permission: string
  group: string
}

/** 全部可导航页面；permission 为访问所需权限码。 */
export const navItems: NavItem[] = [
  { key: 'dashboard', label: '总览', path: '/dashboard', icon: 'grid', permission: 'dashboard:view', group: '工作台' },
  { key: 'users', label: '用户管理', path: '/system/users', icon: 'users', permission: 'system:user:list', group: '人员与权限' },
  { key: 'roles', label: '角色管理', path: '/system/roles', icon: 'shield', permission: 'system:role:list', group: '人员与权限' },
  { key: 'menus', label: '菜单管理', path: '/system/menus', icon: 'tree', permission: 'system:menu:list', group: '人员与权限' },
  { key: 'departments', label: '部门管理', path: '/system/departments', icon: 'building', permission: 'system:dept:list', group: '人员与权限' },
  { key: 'dicts', label: '字典管理', path: '/system/dicts', icon: 'book', permission: 'system:dict:list', group: '系统' },
  { key: 'parameters', label: '参数配置', path: '/system/parameters', icon: 'sliders', permission: 'system:param:list', group: '系统' },
  { key: 'files', label: '文件管理', path: '/system/files', icon: 'folder', permission: 'system:file:list', group: '系统' },
  { key: 'jobs', label: '定时任务', path: '/system/jobs', icon: 'clock', permission: 'system:job:list', group: '系统' },
  { key: 'online', label: '在线用户', path: '/system/online', icon: 'login', permission: 'system:online:list', group: '系统' },
  { key: 'monitor', label: '系统监控', path: '/system/monitor', icon: 'activity', permission: 'system:monitor:view', group: '系统' },
  { key: 'operations', label: '操作日志', path: '/system/logs/operations', icon: 'file', permission: 'system:log:operation', group: '审计' },
  { key: 'logins', label: '登录日志', path: '/system/logs/logins', icon: 'login', permission: 'system:log:login', group: '审计' },
  { key: 'exports', label: '导出记录', path: '/system/exports', icon: 'download', permission: 'system:export:list', group: '审计' },
]

export interface RailGroup {
  key: string
  label: string
  icon: string
  group: string
}

/** 左侧图标栏的一级模块分组。 */
export const railGroups: RailGroup[] = [
  { key: 'workspace', label: '工作台', icon: 'grid', group: '工作台' },
  { key: 'people', label: '人员与权限', icon: 'users', group: '人员与权限' },
  { key: 'system', label: '系统', icon: 'sliders', group: '系统' },
  { key: 'audit', label: '审计', icon: 'file', group: '审计' },
]

/** 取出某个一级模块下的全部页面。 */
export function itemsOfGroup(group: string): NavItem[] {
  return navItems.filter((item) => item.group === group)
}

/** 仪表盘上的快捷操作入口。 */
export const quickActions = [
  { label: '新建用户', path: '/system/users?new=1', icon: 'plus', hint: 'N', permission: 'system:user:create' },
  { label: '分配角色', path: '/system/roles', icon: 'shield', hint: 'R', permission: 'system:role:assign' },
  { label: '上传文件', path: '/system/files', icon: 'upload', hint: 'U', permission: 'system:file:upload' },
  { label: '导出日志', path: '/system/logs/operations', icon: 'download', hint: 'E', permission: 'system:log:operation' },
  { label: '任务调度', path: '/system/jobs', icon: 'clock', hint: 'T', permission: 'system:job:list' },
  { label: '参数配置', path: '/system/parameters', icon: 'settings', hint: 'S', permission: 'system:param:list' },
]

export const profileNav: NavItem = {
  key: 'profile',
  label: '个人中心',
  path: '/profile',
  icon: 'user',
  permission: '',
  group: '账号',
}

/** 返回页面所需权限；首页、个人中心与未知路径不做权限拦截。 */
export function permissionForPath(pathname: string): string {
  if (pathname === '/' || pathname === profileNav.path) return ''
  const item = navItems.find((entry) => pathname === entry.path || pathname.startsWith(`${entry.path}/`))
  return item?.permission ?? ''
}

/** 根据路径匹配导航项，用于面包屑和页签标题。 */
export function navByPath(pathname: string): NavItem {
  if (pathname === profileNav.path) return profileNav
  return (
    navItems.find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`)) ??
    navItems[0]
  )
}


/** 已知路由的元数据索引；后端菜单只负责决定展示哪些页面及其顺序/名称。 */
const navMeta = new Map(navItems.map((item) => [item.path, item]))

/** 后端菜单树转换后的导航视图。 */
export interface NavBundle {
  items: NavItem[]
  groups: RailGroup[]
  byPath: (pathname: string) => NavItem
}

/** 把 /auth/me 返回的菜单树转换为侧边导航。 */
export function buildNav(menus: Menu[], can: (permission?: string) => boolean): NavBundle {
  const items: NavItem[] = []
  const groupOrder: string[] = []
  const groupIcons = new Map<string, string>()
  const groupCounts = new Map<string, number>()

  const addGroup = (group: string, icon: string) => {
    if (!groupOrder.includes(group)) {
      groupOrder.push(group)
      groupIcons.set(group, icon)
    }
  }

  const visit = (nodes: Menu[], group: string, groupIcon: string) => {
    for (const node of nodes) {
      if (node.type === 1) {
        addGroup(node.name, node.icon || groupIcon || 'sliders')
        visit(node.children ?? [], node.name, node.icon || groupIcon)
        continue
      }
      if (node.type !== 2 || !node.visible || node.status !== 1 || !node.path) continue
      const meta = navMeta.get(node.path)
      // 前端路由是静态注册的，未知路径不进入导航，避免点击后 404。
      if (!meta || node.path === '/profile') continue
      if (!can(node.permission || meta.permission)) continue
      const menuGroup = group || '工作台'
      addGroup(menuGroup, groupIcon || meta.icon)
      groupCounts.set(menuGroup, (groupCounts.get(menuGroup) ?? 0) + 1)
      items.push({
        ...meta,
        label: node.name || meta.label,
        icon: node.icon || meta.icon,
        permission: node.permission || meta.permission,
        group: menuGroup,
      })
    }
  }
  visit(menus, '', 'grid')

  // /auth/me 尚未返回时用静态配置兜底；正常情况下始终走后端菜单。
  if (items.length === 0) {
    for (const item of navItems) {
      if (!can(item.permission)) continue
      items.push(item)
      addGroup(item.group, 'grid')
      groupCounts.set(item.group, (groupCounts.get(item.group) ?? 0) + 1)
    }
  }

  const groups: RailGroup[] = groupOrder
    .filter((group) => (groupCounts.get(group) ?? 0) > 0)
    .map((group) => ({
      key: group,
      label: group,
      icon: groupIcons.get(group) || 'grid',
      group,
    }))

  const byPath = (pathname: string): NavItem => {
    if (pathname === profileNav.path) return profileNav
    return (
      items.find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`)) ??
      items[0] ??
      navItems[0]
    )
  }

  return { items, groups, byPath }
}

/** 读取当前用户菜单并转换为导航视图。 */
export function useNav(): NavBundle {
  const menus = useAuth((state) => state.menus)
  const permissions = useAuth((state) => state.permissions)
  const user = useAuth((state) => state.user)

  return useMemo(() => {
    const superUser = !!user?.roles?.some((role) => role.code === 'super_admin')
    const can = (permission?: string) => {
      if (!permission) return true
      return superUser || permissions.includes(permission)
    }
    return buildNav(menus, can)
  }, [menus, permissions, user])
}
