/**
 * 登录令牌的本地持久化（localStorage）。
 */
// localStorage 键名；保持稳定，避免升级后用户被强制退出。
const ACCESS_KEY = 'x-admin.access'
const REFRESH_KEY = 'x-admin.refresh'

/** 令牌读写封装：内存中的字段供请求拦截器同步读取，localStorage 负责刷新后恢复。 */
export const tokenStore = {
  access: localStorage.getItem(ACCESS_KEY) ?? '',
  refresh: localStorage.getItem(REFRESH_KEY) ?? '',
  set(access: string, refresh: string) {
    this.access = access
    this.refresh = refresh
    localStorage.setItem(ACCESS_KEY, access)
    localStorage.setItem(REFRESH_KEY, refresh)
  },
  clear() {
    this.access = ''
    this.refresh = ''
    localStorage.removeItem(ACCESS_KEY)
    localStorage.removeItem(REFRESH_KEY)
  },
}
