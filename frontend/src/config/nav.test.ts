import { describe, expect, it } from 'vitest'
import type { Menu } from '../api/types'
import { buildNav, isKnownPath, navByPath } from './nav'

function menu(overrides: Partial<Menu>): Menu {
  return {
    id: 1,
    parent_id: 0,
    name: '菜单',
    type: 2,
    path: '',
    component: '',
    permission: '',
    icon: '',
    sort: 1,
    visible: true,
    status: 1,
    ...overrides,
  }
}

describe('buildNav', () => {
  it('按后端菜单树生成分组、名称与顺序', () => {
    const menus: Menu[] = [
      menu({ id: 1, name: '总览', path: '/dashboard', permission: 'dashboard:view', icon: 'grid' }),
      menu({
        id: 2,
        name: '系统管理',
        type: 1,
        icon: 'settings',
        children: [
          menu({ id: 10, name: '用户中心', path: '/system/users', permission: 'system:user:list', icon: 'users' }),
          menu({ id: 11, name: '角色中心', path: '/system/roles', permission: 'system:role:list', icon: 'shield' }),
        ],
      }),
    ]
    const nav = buildNav(menus, () => true)
    expect(nav.items.map((item) => item.path)).toEqual(['/dashboard', '/system/users', '/system/roles'])
    expect(nav.items[1].label).toBe('用户中心')
    expect(nav.items[1].group).toBe('系统管理')
    expect(nav.groups.map((group) => group.label)).toEqual(['工作台', '系统管理'])
    expect(nav.groups[1].icon).toBe('settings')
    expect(nav.byPath('/system/roles').label).toBe('角色中心')
  })

  it('过滤无权限、隐藏和未知路径的菜单', () => {
    const menus: Menu[] = [
      menu({ id: 1, name: '总览', path: '/dashboard', permission: 'dashboard:view' }),
      menu({ id: 2, name: '隐藏', path: '/system/users', permission: 'system:user:list', visible: false }),
      menu({ id: 3, name: '未知', path: '/not-registered', permission: 'dashboard:view' }),
      menu({ id: 4, name: '角色', path: '/system/roles', permission: 'system:role:list' }),
    ]
    const nav = buildNav(menus, (permission) => permission === 'dashboard:view')
    expect(nav.items.map((item) => item.path)).toEqual(['/dashboard'])
  })

  it('菜单为空时回退到静态配置', () => {
    const nav = buildNav([], () => true)
    expect(nav.items.length).toBeGreaterThan(0)
    expect(nav.items[0].path).toBe('/dashboard')
  })

  it('未知路径回退到 404 导航项', () => {
    const nav = buildNav([], () => true)
    expect(nav.byPath('/system/monitora').key).toBe('not-found')
    expect(nav.byPath('/system/monitora').label).toBe('页面不存在')
    expect(navByPath('/system/monitora').label).toBe('页面不存在')
    expect(isKnownPath('/system/monitor')).toBe(true)
    expect(isKnownPath('/system/monitora')).toBe(false)
  })
})
