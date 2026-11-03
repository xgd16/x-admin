import type {
  Captcha,
  DashboardSummary,
  Department,
  DictItem,
  DictType,
  FileItem,
  Job,
  LoginLog,
  MeResult,
  Menu,
  OperationLog,
  PageResult,
  Parameter,
  Role,
  User,
} from '../api/types'

function fail(message: string, code = 40401): never {
  throw Object.assign(new Error(message), { code })
}

const permissions = [
  'dashboard:view',
  'system:user:list', 'system:user:create', 'system:user:update', 'system:user:delete',
  'system:user:reset', 'system:user:import', 'system:user:export',
  'system:role:list', 'system:role:create', 'system:role:update', 'system:role:delete', 'system:role:assign',
  'system:menu:list', 'system:menu:create', 'system:menu:update', 'system:menu:delete',
  'system:dept:list', 'system:dept:create', 'system:dept:update', 'system:dept:delete',
  'system:dict:list', 'system:dict:create', 'system:dict:update', 'system:dict:delete',
  'system:param:list', 'system:param:create', 'system:param:update', 'system:param:delete',
  'system:file:list', 'system:file:upload', 'system:file:delete', 'system:file:download',
  'system:job:list', 'system:job:create', 'system:job:update', 'system:job:delete', 'system:job:run',
  'system:log:operation', 'system:log:login',
]

const roles: Role[] = [
  { id: 1, name: '超级管理员', code: 'super_admin', status: 1, sort: 1, remark: '拥有全部权限' },
  { id: 2, name: '运营管理员', code: 'operator', status: 1, sort: 2, remark: '日常运营' },
  { id: 3, name: '财务', code: 'finance', status: 1, sort: 3, remark: '财务报表' },
  { id: 4, name: '编辑', code: 'editor', status: 1, sort: 4, remark: '内容编辑' },
  { id: 5, name: '访客', code: 'guest', status: 0, sort: 5, remark: '只读' },
]

const departments: Department[] = [
  { id: 1, parent_id: 0, name: '总公司', leader: '李经理', phone: '13800000000', email: '', sort: 1, status: 1 },
  { id: 2, parent_id: 1, name: '技术部', leader: '陈默', phone: '13800000001', email: '', sort: 1, status: 1 },
  { id: 3, parent_id: 1, name: '运营部', leader: '林小满', phone: '13800000002', email: '', sort: 2, status: 1 },
  { id: 4, parent_id: 1, name: '财务部', leader: '周予安', phone: '13800000003', email: '', sort: 3, status: 1 },
  { id: 5, parent_id: 1, name: '市场部', leader: '苏念', phone: '13800000004', email: '', sort: 4, status: 1 },
]

const avatarSeed: User[] = [
  { id: 1, username: 'chenmo', nickname: '陈默', email: 'chenmo@x-admin.cn', phone: '13800000001', avatar: '', dept_id: 2, dept: departments[1], status: 1, last_login_at: '2026-11-02T09:42:00+08:00', last_login_ip: '10.20.4.18', remark: '', roles: [roles[0]], created_at: '2025-01-02T09:00:00+08:00' },
  { id: 2, username: 'linxm', nickname: '林小满', email: 'linxm@x-admin.cn', phone: '13800000002', avatar: '', dept_id: 3, dept: departments[2], status: 1, last_login_at: '2026-11-02T09:18:00+08:00', last_login_ip: '10.20.7.32', remark: '', roles: [roles[1]], created_at: '2025-02-08T09:00:00+08:00' },
  { id: 3, username: 'zhouya', nickname: '周予安', email: 'zhouya@x-admin.cn', phone: '13800000003', avatar: '', dept_id: 4, dept: departments[3], status: 1, last_login_at: '2026-11-01T18:06:00+08:00', last_login_ip: '10.20.2.11', remark: '', roles: [roles[2]], created_at: '2025-03-11T09:00:00+08:00' },
  { id: 4, username: 'sunian', nickname: '苏念', email: 'sunian@x-admin.cn', phone: '13800000004', avatar: '', dept_id: 5, dept: departments[4], status: 1, last_login_at: '2026-11-01T17:32:00+08:00', last_login_ip: '10.20.9.27', remark: '', roles: [roles[3]], created_at: '2025-04-19T09:00:00+08:00' },
  { id: 5, username: 'guyan', nickname: '顾言', email: 'guyan@x-admin.cn', phone: '13800000005', avatar: '', dept_id: 2, dept: departments[1], status: 0, last_login_at: '2026-10-31T21:14:00+08:00', last_login_ip: '10.20.4.22', remark: '', roles: [roles[3]], created_at: '2025-05-21T09:00:00+08:00' },
  { id: 6, username: 'xuxh', nickname: '许星河', email: 'xuxh@x-admin.cn', phone: '13800000006', avatar: '', dept_id: 5, dept: departments[4], status: 1, last_login_at: '2026-10-29T20:05:00+08:00', last_login_ip: '10.20.9.31', remark: '', roles: [roles[4]], created_at: '2025-06-03T09:00:00+08:00' },
]

const menus: Menu[] = [
  { id: 1, parent_id: 0, name: '总览', type: 2, path: '/dashboard', component: 'dashboard', permission: 'dashboard:view', icon: 'grid', sort: 1, visible: true, status: 1 },
  { id: 2, parent_id: 0, name: '系统管理', type: 1, path: '/system', component: '', permission: '', icon: 'settings', sort: 2, visible: true, status: 1 },
  { id: 10, parent_id: 2, name: '用户管理', type: 2, path: '/system/users', component: 'system/users', permission: 'system:user:list', icon: 'users', sort: 1, visible: true, status: 1 },
  { id: 11, parent_id: 2, name: '角色管理', type: 2, path: '/system/roles', component: 'system/roles', permission: 'system:role:list', icon: 'shield', sort: 2, visible: true, status: 1 },
  { id: 12, parent_id: 2, name: '菜单管理', type: 2, path: '/system/menus', component: 'system/menus', permission: 'system:menu:list', icon: 'tree', sort: 3, visible: true, status: 1 },
  { id: 13, parent_id: 2, name: '部门管理', type: 2, path: '/system/departments', component: 'system/departments', permission: 'system:dept:list', icon: 'building', sort: 4, visible: true, status: 1 },
  { id: 14, parent_id: 2, name: '字典管理', type: 2, path: '/system/dicts', component: 'system/dicts', permission: 'system:dict:list', icon: 'book', sort: 5, visible: true, status: 1 },
  { id: 15, parent_id: 2, name: '参数配置', type: 2, path: '/system/parameters', component: 'system/parameters', permission: 'system:param:list', icon: 'sliders', sort: 6, visible: true, status: 1 },
  { id: 16, parent_id: 2, name: '文件管理', type: 2, path: '/system/files', component: 'system/files', permission: 'system:file:list', icon: 'folder', sort: 7, visible: true, status: 1 },
  { id: 17, parent_id: 2, name: '定时任务', type: 2, path: '/system/jobs', component: 'system/jobs', permission: 'system:job:list', icon: 'clock', sort: 8, visible: true, status: 1 },
  { id: 18, parent_id: 2, name: '操作日志', type: 2, path: '/system/logs/operations', component: 'system/logs/operations', permission: 'system:log:operation', icon: 'file', sort: 9, visible: true, status: 1 },
  { id: 19, parent_id: 2, name: '登录日志', type: 2, path: '/system/logs/logins', component: 'system/logs/logins', permission: 'system:log:login', icon: 'login', sort: 10, visible: true, status: 1 },
  { id: 20, parent_id: 10, name: '新建用户', type: 3, path: '', component: '', permission: 'system:user:create', icon: '', sort: 1, visible: true, status: 1 },
]

const dictTypes: DictType[] = [
  { id: 1, name: '用户状态', code: 'user_status', status: 1, remark: '' },
  { id: 2, name: '任务状态', code: 'job_status', status: 1, remark: '' },
  { id: 3, name: '菜单类型', code: 'menu_type', status: 1, remark: '' },
]

const dictItems: DictItem[] = [
  { id: 1, dict_type_id: 1, label: '正常', value: '1', sort: 1, status: 1, tag_color: 'green' },
  { id: 2, dict_type_id: 1, label: '停用', value: '0', sort: 2, status: 1, tag_color: 'default' },
  { id: 3, dict_type_id: 2, label: '启用', value: '1', sort: 1, status: 1, tag_color: 'green' },
  { id: 4, dict_type_id: 2, label: '停用', value: '0', sort: 2, status: 1, tag_color: 'default' },
  { id: 5, dict_type_id: 3, label: '目录', value: '1', sort: 1, status: 1, tag_color: 'blue' },
  { id: 6, dict_type_id: 3, label: '菜单', value: '2', sort: 2, status: 1, tag_color: 'green' },
]

const parameters: Parameter[] = [
  { id: 1, key: 'site.name', value: 'x-admin', name: '站点名称', public: true, remark: '' },
  { id: 2, key: 'site.logo', value: '', name: '站点 Logo', public: true, remark: '' },
  { id: 3, key: 'upload.max_size', value: '20', name: '上传大小上限（MB）', public: false, remark: '' },
]

const files: FileItem[] = [
  { id: 1, name: 'brand-guide.pdf', key: '2026/11/a.pdf', url: '/uploads/a.pdf', mime: 'application/pdf', ext: 'pdf', size: 2457600, uploader_id: 1, created_at: '2026-11-02T09:12:00+08:00' },
  { id: 2, name: 'users-template.xlsx', key: '2026/11/b.xlsx', url: '/uploads/b.xlsx', mime: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', ext: 'xlsx', size: 18432, uploader_id: 2, created_at: '2026-11-01T17:20:00+08:00' },
]

const jobs: Job[] = [
  { id: 1, name: '清理过期日志', cron: '0 3 * * *', handler: 'clean_logs', params: '{"days":90}', status: 1, last_run_at: '2026-11-02T03:00:00+08:00', last_result: '成功', remark: '每天 03:00 清理 90 天前日志' },
]

const operationLogs: OperationLog[] = [
  { id: 1, user_id: 1, username: '陈默', method: 'PUT', path: '/admin/v1/roles/2/menus', ip: '10.20.4.18', user_agent: 'Chrome', status: 200, latency_ms: 32, request_id: 'r1', body: '{"menu_ids":[1,2,10]}', error: '', created_at: '2026-11-02T09:42:00+08:00' },
  { id: 2, user_id: 2, username: '林小满', method: 'POST', path: '/admin/v1/users', ip: '10.20.7.32', user_agent: 'Chrome', status: 201, latency_ms: 48, request_id: 'r2', body: '{"username":"xuxh"}', error: '', created_at: '2026-11-02T09:18:00+08:00' },
  { id: 3, user_id: 3, username: '周予安', method: 'GET', path: '/admin/v1/users/export', ip: '10.20.2.11', user_agent: 'Chrome', status: 200, latency_ms: 86, request_id: 'r3', body: '', error: '', created_at: '2026-11-02T09:06:00+08:00' },
  { id: 4, user_id: 4, username: '苏念', method: 'PUT', path: '/admin/v1/dict/types/3', ip: '10.20.9.27', user_agent: 'Chrome', status: 200, latency_ms: 21, request_id: 'r4', body: '{"name":"行业分类"}', error: '', created_at: '2026-11-02T08:52:00+08:00' },
]

const loginLogs: LoginLog[] = [
  { id: 1, user_id: 1, username: 'admin', ip: '10.20.4.18', user_agent: 'Chrome', status: 200, message: '登录成功', created_at: '2026-11-02T09:42:00+08:00' },
  { id: 2, user_id: 2, username: 'linxm', ip: '10.20.7.32', user_agent: 'Safari', status: 200, message: '登录成功', created_at: '2026-11-02T09:18:00+08:00' },
  { id: 3, user_id: 0, username: 'root', ip: '10.20.2.11', user_agent: 'curl', status: 401, message: '用户名或密码错误', created_at: '2026-11-02T08:58:00+08:00' },
]

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function buildTree<T extends { id: number; parent_id: number; children?: T[] }>(rows: T[]): T[] {
  const map = new Map(rows.map((row) => [row.id, { ...row, children: [] as T[] }]))
  const roots: T[] = []
  map.forEach((node) => {
    const parent = map.get(node.parent_id)
    if (node.parent_id === 0 || !parent) roots.push(node)
    else parent.children!.push(node)
  })
  map.forEach((node) => {
    if (node.children && node.children.length === 0) delete (node as { children?: T[] }).children
  })
  return roots
}

function page<T>(rows: T[], params: Record<string, unknown>): PageResult<T> {
  const current = Number(params.page ?? 1)
  const size = Number(params.page_size ?? 20)
  return { list: clone(rows.slice((current - 1) * size, current * size)), total: rows.length, page: current, page_size: size }
}

function matchID(path: string, pattern: RegExp): number {
  const hit = path.match(pattern)
  if (!hit) fail('路径不存在')
  return Number(hit[1])
}

export async function mockRequest<T>(
  method: string,
  path: string,
  params: Record<string, unknown> = {},
  body?: unknown,
): Promise<T> {
  await new Promise((resolve) => setTimeout(resolve, 120))
  const payload = (body ?? {}) as Record<string, unknown>

  if (method === 'GET' && path === '/auth/captcha') {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40"><rect width="120" height="40" rx="10" fill="#eef6fd"/><text x="14" y="28" font-family="Menlo,monospace" font-size="22" font-weight="700" fill="#e2542b">8</text><text x="40" y="28" font-family="Menlo,monospace" font-size="22" font-weight="700" fill="#2f6f9f">4</text><text x="66" y="28" font-family="Menlo,monospace" font-size="22" font-weight="700" fill="#6b4fbb">2</text><text x="92" y="28" font-family="Menlo,monospace" font-size="22" font-weight="700" fill="#187a5a">6</text></svg>`
    return { captcha_id: 'mock', image: `data:image/svg+xml;base64,${btoa(svg)}`, expires_in: 300 } as Captcha as T
  }
  if (method === 'POST' && path === '/auth/login') {
    return { access_token: 'mock-access', refresh_token: 'mock-refresh', token_type: 'Bearer', expires_in: 7200 } as T
  }
  if (method === 'POST' && path === '/auth/refresh') {
    return { access_token: 'mock-access', refresh_token: 'mock-refresh', token_type: 'Bearer', expires_in: 7200 } as T
  }
  if (method === 'POST' && path === '/auth/logout') return null as T
  if (method === 'GET' && path === '/auth/me') {
    return { user: clone(avatarSeed[0]), permissions, menus: clone(menus) } as MeResult as T
  }
  if (method === 'GET' && path === '/dashboard/summary') {
    const summary: DashboardSummary = {
      users: 12846, active_users: 1203, roles: 6, menus: 42, files: 1204, file_size: 5153960755, jobs: 2,
      login_trend: [
        { date: '2026-10-27', count: 420 }, { date: '2026-10-28', count: 468 }, { date: '2026-10-29', count: 512 },
        { date: '2026-10-30', count: 486 }, { date: '2026-10-31', count: 508 }, { date: '2026-11-01', count: 602 },
        { date: '2026-11-02', count: 680 },
      ],
      recent_operations: clone(operationLogs),
    }
    return summary as T
  }

  if (method === 'GET' && path === '/users') {
    let rows = avatarSeed
    if (params.keyword) {
      const keyword = String(params.keyword)
      rows = rows.filter((row) => `${row.username}${row.nickname}${row.email}${row.phone}`.includes(keyword))
    }
    if (params.status !== undefined && params.status !== '') rows = rows.filter((row) => row.status === Number(params.status))
    if (params.dept_id) rows = rows.filter((row) => row.dept_id === Number(params.dept_id))
    return page(rows, params) as T
  }
  if (method === 'POST' && path === '/users') {
    const id = Math.max(...avatarSeed.map((u) => u.id)) + 1
    const roleIDs = Array.isArray(payload.role_ids) ? (payload.role_ids as number[]) : []
    const user: User = {
      id, username: String(payload.username ?? ''), nickname: String(payload.nickname ?? ''), email: String(payload.email ?? ''),
      phone: String(payload.phone ?? ''), avatar: '', dept_id: payload.dept_id ? Number(payload.dept_id) : null,
      status: Number(payload.status ?? 1) as 0 | 1, last_login_at: null, last_login_ip: '', remark: String(payload.remark ?? ''),
      roles: roles.filter((role) => roleIDs.includes(role.id)), created_at: new Date().toISOString(),
    }
    user.dept = departments.find((dept) => dept.id === user.dept_id)
    avatarSeed.unshift(user)
    return clone(user) as T
  }
  if (method === 'PUT' && /^\/users\/\d+$/.test(path)) {
    const id = matchID(path, /^\/users\/(\d+)$/)
    const user = avatarSeed.find((row) => row.id === id) ?? fail('用户不存在')
    Object.assign(user, {
      nickname: payload.nickname ?? user.nickname, email: payload.email ?? user.email, phone: payload.phone ?? user.phone,
      status: Number(payload.status ?? user.status) as 0 | 1,
      dept_id: payload.dept_id === undefined ? user.dept_id : payload.dept_id ? Number(payload.dept_id) : null,
    })
    if (Array.isArray(payload.role_ids)) user.roles = roles.filter((role) => (payload.role_ids as number[]).includes(role.id))
    user.dept = departments.find((dept) => dept.id === user.dept_id)
    return clone(user) as T
  }
  if (method === 'PUT' && /^\/users\/\d+\/status$/.test(path)) {
    const id = matchID(path, /^\/users\/(\d+)\/status$/)
    const user = avatarSeed.find((row) => row.id === id) ?? fail('用户不存在')
    user.status = Number(payload.status) as 0 | 1
    return null as T
  }
  if (method === 'PUT' && /^\/users\/\d+\/password$/.test(path)) return null as T
  if (method === 'PUT' && /^\/users\/\d+\/roles$/.test(path)) return null as T
  if (method === 'DELETE' && path === '/users') {
    const ids = (payload.ids as number[]) ?? []
    ids.forEach((id) => {
      const index = avatarSeed.findIndex((row) => row.id === id)
      if (index >= 0) avatarSeed.splice(index, 1)
    })
    return null as T
  }
  if (method === 'POST' && path === '/users/import') return { success: 1, failed: [] } as T

  if (method === 'GET' && (path === '/roles' || path === '/roles/all')) return clone(roles) as T
  if (method === 'POST' && path === '/roles') {
    const role: Role = { id: Math.max(...roles.map((r) => r.id)) + 1, name: String(payload.name), code: String(payload.code), status: Number(payload.status ?? 1) as 0 | 1, sort: Number(payload.sort ?? 0), remark: String(payload.remark ?? '') }
    roles.push(role)
    return clone(role) as T
  }
  if (method === 'PUT' && /^\/roles\/\d+$/.test(path)) {
    const role = roles.find((row) => row.id === matchID(path, /^\/roles\/(\d+)$/)) ?? fail('角色不存在')
    Object.assign(role, { name: payload.name ?? role.name, code: payload.code ?? role.code, status: Number(payload.status ?? role.status), sort: Number(payload.sort ?? role.sort), remark: payload.remark ?? role.remark })
    return clone(role) as T
  }
  if (method === 'GET' && /^\/roles\/\d+\/menus$/.test(path)) return [1, 2, 10, 18, 19] as T
  if (method === 'PUT' && /^\/roles\/\d+\/menus$/.test(path)) return null as T
  if (method === 'DELETE' && /^\/roles\/\d+$/.test(path)) {
    const index = roles.findIndex((row) => row.id === matchID(path, /^\/roles\/(\d+)$/))
    if (index >= 0) roles.splice(index, 1)
    return null as T
  }

  if (method === 'GET' && path === '/menus/tree') return buildTree(clone(menus)) as T
  if (method === 'POST' && path === '/menus') {
    const menu: Menu = { id: Math.max(...menus.map((m) => m.id)) + 1, parent_id: Number(payload.parent_id ?? 0), name: String(payload.name), type: Number(payload.type ?? 2) as 1 | 2 | 3, path: String(payload.path ?? ''), component: String(payload.component ?? ''), permission: String(payload.permission ?? ''), icon: String(payload.icon ?? ''), sort: Number(payload.sort ?? 0), visible: Boolean(payload.visible ?? true), status: Number(payload.status ?? 1) as 0 | 1 }
    menus.push(menu)
    return clone(menu) as T
  }
  if (method === 'PUT' && /^\/menus\/\d+$/.test(path)) {
    const menu = menus.find((row) => row.id === matchID(path, /^\/menus\/(\d+)$/)) ?? fail('菜单不存在')
    Object.assign(menu, payload)
    return clone(menu) as T
  }
  if (method === 'DELETE' && /^\/menus\/\d+$/.test(path)) {
    const index = menus.findIndex((row) => row.id === matchID(path, /^\/menus\/(\d+)$/))
    if (index >= 0) menus.splice(index, 1)
    return null as T
  }

  if (method === 'GET' && path === '/departments/tree') return buildTree(clone(departments)) as T
  if (method === 'POST' && path === '/departments') {
    const dept: Department = { id: Math.max(...departments.map((d) => d.id)) + 1, parent_id: Number(payload.parent_id ?? 0), name: String(payload.name), leader: String(payload.leader ?? ''), phone: String(payload.phone ?? ''), email: String(payload.email ?? ''), sort: Number(payload.sort ?? 0), status: Number(payload.status ?? 1) as 0 | 1 }
    departments.push(dept)
    return clone(dept) as T
  }
  if (method === 'PUT' && /^\/departments\/\d+$/.test(path)) {
    const dept = departments.find((row) => row.id === matchID(path, /^\/departments\/(\d+)$/)) ?? fail('部门不存在')
    Object.assign(dept, payload)
    return clone(dept) as T
  }
  if (method === 'DELETE' && /^\/departments\/\d+$/.test(path)) return null as T

  if (method === 'GET' && path === '/dict/types') return page(dictTypes, params) as T
  if (method === 'POST' && path === '/dict/types') {
    const item: DictType = { id: Math.max(...dictTypes.map((d) => d.id)) + 1, name: String(payload.name), code: String(payload.code), status: Number(payload.status ?? 1) as 0 | 1, remark: String(payload.remark ?? '') }
    dictTypes.push(item)
    return clone(item) as T
  }
  if (method === 'PUT' && /^\/dict\/types\/\d+$/.test(path)) {
    const item = dictTypes.find((row) => row.id === matchID(path, /^\/dict\/types\/(\d+)$/)) ?? fail('字典不存在')
    Object.assign(item, payload)
    return clone(item) as T
  }
  if (method === 'DELETE' && /^\/dict\/types\/\d+$/.test(path)) return null as T
  if (method === 'GET' && /^\/dict\/types\/\d+\/items$/.test(path)) {
    const typeID = matchID(path, /^\/dict\/types\/(\d+)\/items$/)
    return clone(dictItems.filter((item) => item.dict_type_id === typeID)) as T
  }
  if (method === 'POST' && /^\/dict\/types\/\d+\/items$/.test(path)) {
    const typeID = matchID(path, /^\/dict\/types\/(\d+)\/items$/)
    const item: DictItem = { id: Math.max(...dictItems.map((d) => d.id)) + 1, dict_type_id: typeID, label: String(payload.label), value: String(payload.value), sort: Number(payload.sort ?? 0), status: Number(payload.status ?? 1) as 0 | 1, tag_color: String(payload.tag_color ?? '') }
    dictItems.push(item)
    return clone(item) as T
  }
  if (method === 'PUT' && /^\/dict\/items\/\d+$/.test(path)) {
    const item = dictItems.find((row) => row.id === matchID(path, /^\/dict\/items\/(\d+)$/)) ?? fail('字典项不存在')
    Object.assign(item, payload)
    return clone(item) as T
  }
  if (method === 'DELETE' && /^\/dict\/items\/\d+$/.test(path)) return null as T

  if (method === 'GET' && path === '/parameters') return page(parameters, params) as T
  if (method === 'POST' && path === '/parameters') {
    const item: Parameter = { id: Math.max(...parameters.map((p) => p.id)) + 1, key: String(payload.key), value: String(payload.value ?? ''), name: String(payload.name), public: Boolean(payload.public), remark: String(payload.remark ?? '') }
    parameters.push(item)
    return clone(item) as T
  }
  if (method === 'PUT' && /^\/parameters\/\d+$/.test(path)) {
    const item = parameters.find((row) => row.id === matchID(path, /^\/parameters\/(\d+)$/)) ?? fail('参数不存在')
    Object.assign(item, payload)
    return clone(item) as T
  }
  if (method === 'DELETE' && /^\/parameters\/\d+$/.test(path)) return null as T

  if (method === 'GET' && path === '/files') return page(files, params) as T
  if (method === 'POST' && path === '/files') {
    const item: FileItem = { id: Math.max(...files.map((f) => f.id)) + 1, name: 'upload.bin', key: `2026/11/${Date.now()}.bin`, url: '/uploads/mock.bin', mime: 'application/octet-stream', ext: 'bin', size: 1024, uploader_id: 1, created_at: new Date().toISOString() }
    files.unshift(item)
    return clone(item) as T
  }
  if (method === 'DELETE' && /^\/files\/\d+$/.test(path)) {
    const index = files.findIndex((row) => row.id === matchID(path, /^\/files\/(\d+)$/))
    if (index >= 0) files.splice(index, 1)
    return null as T
  }

  if (method === 'GET' && path === '/jobs') return page(jobs, params) as T
  if (method === 'POST' && path === '/jobs') {
    const item: Job = { id: Math.max(...jobs.map((j) => j.id)) + 1, name: String(payload.name), cron: String(payload.cron), handler: String(payload.handler), params: String(payload.params ?? ''), status: Number(payload.status ?? 1) as 0 | 1, last_run_at: null, last_result: '', remark: String(payload.remark ?? '') }
    jobs.push(item)
    return clone(item) as T
  }
  if (method === 'PUT' && /^\/jobs\/\d+$/.test(path)) {
    const item = jobs.find((row) => row.id === matchID(path, /^\/jobs\/(\d+)$/)) ?? fail('任务不存在')
    Object.assign(item, payload, { status: Number(payload.status ?? item.status) })
    return clone(item) as T
  }
  if (method === 'PUT' && /^\/jobs\/\d+\/status$/.test(path)) {
    const item = jobs.find((row) => row.id === matchID(path, /^\/jobs\/(\d+)\/status$/)) ?? fail('任务不存在')
    item.status = Number(payload.status) as 0 | 1
    return null as T
  }
  if (method === 'POST' && /^\/jobs\/\d+\/run$/.test(path)) return null as T
  if (method === 'DELETE' && /^\/jobs\/\d+$/.test(path)) return null as T

  if (method === 'GET' && path === '/logs/operations') return page(operationLogs, params) as T
  if (method === 'GET' && path === '/logs/logins') return page(loginLogs, params) as T
  if (method === 'DELETE' && path === '/logs/operations') {
    const ids = (payload.ids as number[]) ?? []
    ids.forEach((id) => {
      const index = operationLogs.findIndex((row) => row.id === id)
      if (index >= 0) operationLogs.splice(index, 1)
    })
    return null as T
  }
  if (method === 'DELETE' && path === '/logs/operations/clear') { operationLogs.length = 0; return null as T }
  if (method === 'DELETE' && path === '/logs/logins/clear') { loginLogs.length = 0; return null as T }

  if (method === 'GET' && path === '/profile') return clone(avatarSeed[0]) as T
  if (method === 'PUT' && path === '/profile') {
    Object.assign(avatarSeed[0], payload)
    return clone(avatarSeed[0]) as T
  }
  if (method === 'PUT' && path === '/profile/password') return null as T

  return fail(`Mock 未实现：${method} ${path}`)
}
