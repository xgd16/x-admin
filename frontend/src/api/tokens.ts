const ACCESS_KEY = 'x-admin.access'
const REFRESH_KEY = 'x-admin.refresh'

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
