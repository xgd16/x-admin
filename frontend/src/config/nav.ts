/**
 * 侧边导航、模块分组、快捷操作与路由匹配配置。
 */
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
  { key: 'operations', label: '操作日志', path: '/system/logs/operations', icon: 'file', permission: 'system:log:operation', group: '审计' },
  { key: 'logins', label: '登录日志', path: '/system/logs/logins', icon: 'login', permission: 'system:log:login', group: '审计' },
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
  { label: '新建用户', path: '/system/users?new=1', icon: 'plus', hint: 'N' },
  { label: '分配角色', path: '/system/roles', icon: 'shield', hint: 'R' },
  { label: '上传文件', path: '/system/files', icon: 'upload', hint: 'U' },
  { label: '导出日志', path: '/system/logs/operations', icon: 'download', hint: 'E' },
  { label: '任务调度', path: '/system/jobs', icon: 'clock', hint: 'T' },
  { label: '参数配置', path: '/system/parameters', icon: 'settings', hint: 'S' },
]

export const profileNav: NavItem = {
  key: 'profile',
  label: '个人中心',
  path: '/profile',
  icon: 'user',
  permission: '',
  group: '账号',
}

/** 根据路径匹配导航项，用于面包屑和页签标题。 */
export function navByPath(pathname: string): NavItem {
  if (pathname === profileNav.path) return profileNav
  return (
    navItems.find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`)) ??
    navItems[0]
  )
}
