/**
 * 后端接口的数据模型定义。
 */
// —— 通用响应结构 ——

/** 后端统一响应包裹：code 为 0 表示成功，data 为业务数据。 */
export interface Envelope<T> {
  code: number
  message: string
  data: T
  request_id?: string
}

/** 分页列表的通用返回结构。 */
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

// —— 用户、角色、菜单与部门 ——

/** 用户账号；status 0=停用，1=正常。 */
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
  totp_enabled: boolean
  roles?: Role[]
  created_at: string
}

/** 角色；code 为权限系统中的稳定标识。 */
export interface Role {
  id: number
  name: string
  code: string
  status: 0 | 1
  sort: number
  remark: string
  menus?: Menu[]
}

/** 菜单节点；type 1=目录，2=菜单，3=按钮。 */
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

/** 部门；children 用于树形展示。 */
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

// —— 字典、参数、文件、任务与日志 ——

/** 字典类型，例如“用户状态”。 */
export interface DictType {
  id: number
  name: string
  code: string
  status: 0 | 1
  remark: string
}

/** 字典项，例如“正常 / 停用”。 */
export interface DictItem {
  id: number
  dict_type_id: number
  label: string
  value: string
  sort: number
  status: 0 | 1
  tag_color: string
}

/** 系统参数；public 为 true 时允许未登录页面读取。 */
export interface Parameter {
  id: number
  key: string
  value: string
  name: string
  public: boolean
  remark: string
}

/** 文件库中的文件记录。 */
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

/** 定时任务；handler 对应后端注册的执行器。 */
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

/** 操作日志；请求体与错误信息已由后端脱敏。 */
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

/** 登录日志，记录成功与失败。 */
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

/** 站内通知；read_at 为空表示未读。 */
export interface Notification {
  id: number
  user_id: number
  title: string
  content: string
  type: 'info' | 'success' | 'warning' | 'error'
  link: string
  actor_id: number
  read_at: string | null
  created_at: string
}

// —— 仪表盘与认证 ——

/** 登录趋势中的一个数据点。 */
export interface TrendPoint {
  date: string
  count: number
}

/** 首页汇总数据。 */
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

/** /auth/me 返回当前用户、权限码与可见菜单。 */
export interface MeResult {
  user: User
  permissions: string[]
  menus: Menu[]
}

/** 可撤销的文件分享链接。 */
export interface FileShare {
  id: number
  file_id: number
  created_by: number
  expires_at: string
  revoked_at: string | null
  created_at: string
}

/** 一次任务执行记录。 */
export interface JobRun {
  id: number
  job_id: number
  job_name: string
  trigger: 'manual' | 'cron'
  status: 0 | 1
  result: string
  started_at: string
  finished_at: string
  duration_ms: number
}

/** 在线登录设备。 */
export interface LoginSession {
  id: string
  ip: string
  user_agent: string
  device: string
  current: boolean
  remember: boolean
  created_at: string
  last_seen_at: string
}

/** 图形验证码；image 为 data URL。 */
export interface Captcha {
  captcha_id: string
  image: string
  expires_in: number
}

/** 访问令牌与刷新令牌。 */
export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
}

/** 登录结果；mfa_required 为 true 时需继续提交动态验证码。 */
export interface LoginResult extends Partial<TokenPair> {
  mfa_required?: boolean
  mfa_token?: string
  mfa_expires_in?: number
}

/** TOTP 状态与剩余恢复码数量。 */
export interface TOTPStatus {
  enabled: boolean
  recovery_codes_left: number
}

/** TOTP 绑定信息；qr_code 为二维码 data URL。 */
export interface TOTPSetup {
  secret: string
  otpauth_url: string
  qr_code: string
  issuer: string
  account: string
}

// —— WebAuthn / 通行密钥（服务端 JSON 与浏览器 API 的类型桥接） ——

export interface PasskeyCredentialDescriptorJSON {
  id: string
  type?: string
  transports?: string[]
}

export interface PasskeyCreationOptionsJSON {
  rp: { id: string; name: string }
  user: { id: string; name: string; displayName: string }
  challenge: string
  pubKeyCredParams: Array<{ type: string; alg: number }>
  timeout?: number
  excludeCredentials?: PasskeyCredentialDescriptorJSON[]
  authenticatorSelection?: {
    authenticatorAttachment?: string
    requireResidentKey?: boolean
    residentKey?: string
    userVerification?: string
  }
  attestation?: string
}

export interface PasskeyRequestOptionsJSON {
  challenge: string
  timeout?: number
  rpId?: string
  allowCredentials?: PasskeyCredentialDescriptorJSON[]
  userVerification?: string
}

/** 注册通行密钥的会话信息与 WebAuthn 选项。 */
export interface PasskeyRegistrationOptions {
  session_id: string
  options: { publicKey: PasskeyCreationOptionsJSON; mediation?: string }
}

/** 通行密钥登录的会话信息与 WebAuthn 选项。 */
export interface PasskeyLoginOptions {
  session_id: string
  options: { publicKey: PasskeyRequestOptionsJSON; mediation?: string }
}

/** 已绑定的通行密钥设备。 */
export interface PasskeyItem {
  id: number
  name: string
  created_at: string
  updated_at: string
  last_used_at: string | null
}

/** Excel 导入结果。 */
export interface ImportResult {
  success: number
  failed: Array<{ row: number; username: string; message: string }>
}
