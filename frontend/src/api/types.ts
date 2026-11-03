export interface Envelope<T> {
  code: number
  message: string
  data: T
  request_id?: string
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface User {
  id: number
  username: string
  nickname: string
  email: string
  phone: string
  avatar: string
  dept_id: number | null
  dept?: Department
  status: 0 | 1
  last_login_at: string | null
  last_login_ip: string
  remark: string
  roles?: Role[]
  created_at: string
}

export interface Role {
  id: number
  name: string
  code: string
  status: 0 | 1
  sort: number
  remark: string
  menus?: Menu[]
}

export interface Menu {
  id: number
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
  children?: Menu[]
}

export interface Department {
  id: number
  parent_id: number
  name: string
  leader: string
  phone: string
  email: string
  sort: number
  status: 0 | 1
  children?: Department[]
}

export interface DictType {
  id: number
  name: string
  code: string
  status: 0 | 1
  remark: string
}

export interface DictItem {
  id: number
  dict_type_id: number
  label: string
  value: string
  sort: number
  status: 0 | 1
  tag_color: string
}

export interface Parameter {
  id: number
  key: string
  value: string
  name: string
  public: boolean
  remark: string
}

export interface FileItem {
  id: number
  name: string
  key: string
  url: string
  mime: string
  ext: string
  size: number
  uploader_id: number
  created_at: string
}

export interface Job {
  id: number
  name: string
  cron: string
  handler: string
  params: string
  status: 0 | 1
  last_run_at: string | null
  last_result: string
  remark: string
}

export interface OperationLog {
  id: number
  user_id: number
  username: string
  method: string
  path: string
  ip: string
  user_agent: string
  status: number
  latency_ms: number
  request_id: string
  body: string
  error: string
  created_at: string
}

export interface LoginLog {
  id: number
  user_id: number
  username: string
  ip: string
  user_agent: string
  status: number
  message: string
  created_at: string
}

export interface TrendPoint {
  date: string
  count: number
}

export interface DashboardSummary {
  users: number
  active_users: number
  roles: number
  menus: number
  files: number
  file_size: number
  jobs: number
  login_trend: TrendPoint[]
  recent_operations: OperationLog[]
}

export interface MeResult {
  user: User
  permissions: string[]
  menus: Menu[]
}

export interface Captcha {
  captcha_id: string
  image: string
  expires_in: number
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
}

export interface ImportResult {
  success: number
  failed: Array<{ row: number; username: string; message: string }>
}
