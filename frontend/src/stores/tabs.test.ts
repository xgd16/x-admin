import { beforeEach, describe, expect, it } from 'vitest'
import { useTabs, type Tab } from './tabs'

const dashboard: Tab = { key: '/dashboard', label: '总览', path: '/dashboard' }
const users: Tab = { key: '/system/users', label: '用户管理', path: '/system/users' }
const roles: Tab = { key: '/system/roles', label: '角色管理', path: '/system/roles' }
const logs: Tab = { key: '/system/logs', label: '操作日志', path: '/system/logs' }

function store() {
  return useTabs.getState()
}

function seed(tabs: Tab[], active = tabs[0]?.key ?? '') {
  useTabs.setState({ tabs, active })
}

beforeEach(() => {
  useTabs.setState({ tabs: [], active: '', refreshKey: 0 })
})

describe('tab open and activate', () => {
  it('adds a tab and activates it', () => {
    store().open(dashboard)
    store().open(users)

    expect(store().tabs).toEqual([dashboard, users])
    expect(store().active).toBe(users.key)
  })

  it('does not duplicate an existing tab', () => {
    store().open(dashboard)
    store().open(users)
    store().open({ ...users, label: '用户' })

    expect(store().tabs).toHaveLength(2)
    expect(store().tabs[1].label).toBe('用户管理')
    expect(store().active).toBe(users.key)
  })

  it('activates a known key', () => {
    seed([dashboard, users], dashboard.key)
    store().activate(users.key)
    expect(store().active).toBe(users.key)
  })
})

describe('close', () => {
  it('returns null for an unknown tab and keeps state', () => {
    seed([dashboard], dashboard.key)
    expect(store().close('/missing')).toBeNull()
    expect(store().tabs).toEqual([dashboard])
  })

  it('closes a background tab without changing the active one', () => {
    seed([dashboard, users, roles], users.key)
    expect(store().close(roles.key)).toBeNull()
    expect(store().tabs).toEqual([dashboard, users])
    expect(store().active).toBe(users.key)
  })

  it('selects the next tab when the active one closes', () => {
    seed([dashboard, users, roles], users.key)
    expect(store().close(users.key)).toEqual(roles)
    expect(store().active).toBe(roles.key)
  })

  it('falls back to the previous tab when the active last one closes', () => {
    seed([dashboard, users, roles], roles.key)
    expect(store().close(roles.key)).toEqual(users)
    expect(store().active).toBe(users.key)
  })

  it('empties the store when the only tab closes', () => {
    seed([dashboard], dashboard.key)
    expect(store().close(dashboard.key)).toBeNull()
    expect(store().tabs).toEqual([])
    expect(store().active).toBe('')
  })
})

describe('closeOthers', () => {
  it('keeps only the target and activates it', () => {
    seed([dashboard, users, roles], users.key)
    expect(store().closeOthers(roles.key)).toEqual(roles)
    expect(store().tabs).toEqual([roles])
    expect(store().active).toBe(roles.key)
  })

  it('returns null when the target is already active or missing', () => {
    seed([dashboard, users], users.key)
    expect(store().closeOthers(users.key)).toBeNull()
    expect(store().tabs).toEqual([users])
    expect(store().closeOthers('/missing')).toBeNull()
    expect(store().tabs).toEqual([users])
  })
})

describe('closeLeft', () => {
  it('drops tabs before the target and keeps a surviving active tab', () => {
    seed([dashboard, users, roles], roles.key)
    expect(store().closeLeft(users.key)).toBeNull()
    expect(store().tabs).toEqual([users, roles])
    expect(store().active).toBe(roles.key)
  })

  it('activates the target when the active tab was removed', () => {
    seed([dashboard, users, roles], dashboard.key)
    expect(store().closeLeft(roles.key)).toEqual(roles)
    expect(store().tabs).toEqual([roles])
    expect(store().active).toBe(roles.key)
  })

  it('returns null for the first tab or an unknown key', () => {
    seed([dashboard, users], users.key)
    expect(store().closeLeft(dashboard.key)).toBeNull()
    expect(store().closeLeft('/missing')).toBeNull()
    expect(store().tabs).toEqual([dashboard, users])
    expect(store().active).toBe(users.key)
  })
})

describe('closeRight', () => {
  it('drops tabs after the target', () => {
    seed([dashboard, users, roles, logs], logs.key)
    expect(store().closeRight(users.key)).toEqual(users)
    expect(store().tabs).toEqual([dashboard, users])
    expect(store().active).toBe(users.key)
  })

  it('keeps the active tab when it is on the left', () => {
    seed([dashboard, users, roles], dashboard.key)
    expect(store().closeRight(users.key)).toBeNull()
    expect(store().tabs).toEqual([dashboard, users])
    expect(store().active).toBe(dashboard.key)
  })

  it('returns null when there is nothing to the right or the key is unknown', () => {
    seed([dashboard, users], users.key)
    expect(store().closeRight(users.key)).toBeNull()
    expect(store().closeRight('/missing')).toBeNull()
  })
})

describe('closeAll', () => {
  it('keeps the dashboard and activates it', () => {
    seed([dashboard, users, roles], roles.key)
    expect(store().closeAll()).toEqual(dashboard)
    expect(store().tabs).toEqual([dashboard])
    expect(store().active).toBe(dashboard.key)
  })

  it('empties the store when the dashboard is not open', () => {
    seed([users, roles], roles.key)
    expect(store().closeAll()).toBeNull()
    expect(store().tabs).toEqual([])
    expect(store().active).toBe('')
  })
})

describe('refresh and reset', () => {
  it('advances the remount key', () => {
    store().refresh()
    store().refresh()
    expect(store().refreshKey).toBe(2)
  })

  it('resets tabs and the active key', () => {
    seed([dashboard, users], users.key)
    store().reset()
    expect(store().tabs).toEqual([])
    expect(store().active).toBe('')
  })
})
