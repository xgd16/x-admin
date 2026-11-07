/**
 * 菜单树拖拽排序的纯函数测试。
 */
import { describe, expect, it } from 'vitest'
import type { Menu } from '../api/types'
import { findMenu, menuContainsId, moveMenuNode } from './menuTree'

function menu(id: number, name: string, parentId = 0, children: Menu[] = [], type?: 1 | 2 | 3): Menu {
  return {
    id,
    parent_id: parentId,
    name,
    type: type ?? (children.length > 0 ? 1 : 2),
    path: '',
    component: '',
    permission: '',
    icon: '',
    sort: 0,
    visible: true,
    status: 1,
    children: children.length > 0 ? children : undefined,
  }
}

const tree: Menu[] = [
  menu(1, '工作台', 0, [
    menu(2, '总览', 1),
    menu(3, '个人中心', 1),
  ]),
  menu(4, '人员与权限', 0, [
    menu(5, '用户管理', 4, [menu(6, '新建用户', 5, [], 3)]),
    menu(7, '角色管理', 4),
  ]),
  menu(8, '系统', 0, [menu(9, '文件管理', 8)]),
]

/** 把树拍平成 id 顺序，方便断言。 */
function ids(nodes: Menu[]): number[] {
  return nodes.map((node) => node.id)
}

describe('menuTree', () => {
  it('查找节点并判断后代关系', () => {
    expect(findMenu(tree, 6)?.name).toBe('新建用户')
    expect(findMenu(tree, 999)).toBeUndefined()
    expect(menuContainsId(tree[1], 6)).toBe(true)
    expect(menuContainsId(tree[1], 9)).toBe(false)
  })

  it('同级拖拽后重新排序并只更新受影响的节点', () => {
    const result = moveMenuNode(tree, 3, 2, -1, true)

    expect(result).not.toBeNull()
    expect(ids(result!.tree[0].children ?? [])).toEqual([3, 2])
    expect(result!.tree[0].children?.[0].sort).toBe(0)
    expect(result!.tree[0].children?.[1].sort).toBe(1)
    expect(result!.updates.map((update) => update.menu.id).sort()).toEqual([2, 3])
    expect(result!.updates.every((update) => update.parentId === 1)).toBe(true)
  })

  it('拖入另一个父节点时同步更新 parent_id 与 sort', () => {
    const result = moveMenuNode(tree, 7, 1, 0, false)

    expect(result).not.toBeNull()
    expect(ids(result!.tree[0].children ?? [])).toEqual([2, 3, 7])
    const moved = findMenu(result!.tree, 7)
    expect(moved?.parent_id).toBe(1)
    expect(moved?.sort).toBe(2)
    expect(result!.updates.some((update) => update.menu.id === 7 && update.parentId === 1)).toBe(true)
  })

  it('拖到根节点前后时移动到根级', () => {
    const result = moveMenuNode(tree, 5, 8, 1, true)

    expect(result).not.toBeNull()
    expect(ids(result!.tree)).toEqual([1, 4, 8, 5])
    expect(findMenu(result!.tree, 5)?.parent_id).toBe(0)
    expect(findMenu(result!.tree, 5)?.sort).toBe(3)
  })

  it('拒绝拖到自己、自己的子孙或按钮节点内部', () => {
    expect(moveMenuNode(tree, 1, 1, 0, false)).toBeNull()
    expect(moveMenuNode(tree, 1, 6, 0, false)).toBeNull()
    expect(moveMenuNode(tree, 2, 6, 0, false)).toBeNull()
  })

  it('拖拽后不修改原始树', () => {
    const before = JSON.stringify(tree)
    moveMenuNode(tree, 3, 2, -1, true)
    expect(JSON.stringify(tree)).toBe(before)
  })
})
