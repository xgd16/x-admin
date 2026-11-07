/**
 * 菜单树拖拽排序：把 antd Tree 的落点转换为新的树结构和需要落库的层级/排序变更。
 */
import type { Menu } from '../api/types'

export interface MenuOrderUpdate {
  menu: Menu
  parentId: number
  sort: number
}

export interface MenuMoveResult {
  tree: Menu[]
  updates: MenuOrderUpdate[]
}

interface MenuPosition {
  parentId: number
  index: number
}

type DropMode = 'before' | 'after' | 'inside'

/** 在树中查找菜单节点。 */
export function findMenu(nodes: Menu[], id: number): Menu | undefined {
  for (const node of nodes) {
    if (node.id === id) return node
    const child = node.children ? findMenu(node.children, id) : undefined
    if (child) return child
  }
  return undefined
}

/** 判断某个节点是否包含指定后代（包含自身）。 */
export function menuContainsId(menu: Menu, id: number): boolean {
  if (menu.id === id) return true
  return (menu.children ?? []).some((child) => menuContainsId(child, id))
}

/** 从树中摘除节点，返回新的树和被摘除的节点。 */
function detachMenu(nodes: Menu[], id: number): { nodes: Menu[]; menu?: Menu } {
  const next: Menu[] = []
  let removed: Menu | undefined

  for (const node of nodes) {
    if (node.id === id) {
      removed = node
      continue
    }
    if (node.children?.length) {
      const childResult = detachMenu(node.children, id)
      if (childResult.menu) {
        removed = childResult.menu
        next.push({ ...node, children: childResult.nodes })
      } else {
        next.push(node)
      }
    } else {
      next.push(node)
    }
  }

  return { nodes: next, menu: removed }
}

/** 把节点插入目标节点前、后或内部。 */
function insertMenu(
  nodes: Menu[],
  targetId: number,
  menu: Menu,
  mode: DropMode,
  parentId = 0,
): { nodes: Menu[]; parentId: number; index: number } | null {
  const targetIndex = nodes.findIndex((node) => node.id === targetId)

  if (targetIndex >= 0) {
    if (mode === 'inside') {
      const target = nodes[targetIndex]
      const children = [...(target.children ?? []), menu]
      const next = [...nodes]
      next[targetIndex] = { ...target, children }
      return { nodes: next, parentId: target.id, index: children.length - 1 }
    }

    const next = [...nodes]
    const index = mode === 'before' ? targetIndex : targetIndex + 1
    next.splice(index, 0, menu)
    return { nodes: next, parentId, index }
  }

  for (let position = 0; position < nodes.length; position += 1) {
    const node = nodes[position]
    if (!node.children?.length) continue
    const childResult = insertMenu(node.children, targetId, menu, mode, node.id)
    if (!childResult) continue
    const next = [...nodes]
    next[position] = { ...node, children: childResult.nodes }
    return { nodes: next, parentId: childResult.parentId, index: childResult.index }
  }

  return null
}

/** 设置指定节点的上级 ID，其余字段保持不变。 */
function setMenuParent(nodes: Menu[], id: number, parentId: number): Menu[] {
  return nodes.map((node) => {
    if (node.id === id) return { ...node, parent_id: parentId }
    if (!node.children?.length) return node
    return { ...node, children: setMenuParent(node.children, id, parentId) }
  })
}

/** 记录原始位置，用于判断哪些节点的层级或顺序真正发生了变化。 */
function collectPositions(nodes: Menu[], parentId: number, positions = new Map<number, MenuPosition>()) {
  nodes.forEach((node, index) => {
    positions.set(node.id, { parentId, index })
    if (node.children?.length) collectPositions(node.children, node.id, positions)
  })
  return positions
}

/**
 * 把树按当前数组顺序重新编号。
 * 只有上级或同级位置发生变化的节点才会进入 updates，避免整棵树都被写一遍。
 */
function normalizeOrder(
  original: Menu[],
  next: Menu[],
): MenuMoveResult {
  const positions = collectPositions(original, 0)
  const updates: MenuOrderUpdate[] = []

  const walk = (nodes: Menu[], parentId: number): Menu[] =>
    nodes.map((node, index) => {
      const previous = positions.get(node.id)
      const changed = !previous || previous.parentId !== parentId || previous.index !== index
      const normalized: Menu = { ...node, parent_id: parentId, sort: index }
      if (changed) updates.push({ menu: normalized, parentId, sort: index })
      if (normalized.children?.length) {
        normalized.children = walk(normalized.children, normalized.id)
      }
      return normalized
    })

  return { tree: walk(next, 0), updates }
}

/**
 * 根据 antd Tree 的落点移动节点。
 *
 * - `dropToGap=true`：放在目标节点前面或后面，成为同级节点
 * - `dropToGap=false`：放进目标节点内部
 *
 * 非法移动（自己、子孙、按钮节点内部）返回 null。
 */
export function moveMenuNode(
  tree: Menu[],
  dragId: number,
  dropId: number,
  dropPosition: number,
  dropToGap: boolean,
): MenuMoveResult | null {
  if (dragId === dropId) return null

  const dragMenu = findMenu(tree, dragId)
  const dropMenu = findMenu(tree, dropId)
  if (!dragMenu || !dropMenu) return null
  // 不能把父节点拖进自己的子树，否则会形成环。
  if (menuContainsId(dragMenu, dropId)) return null
  // 按钮是叶子节点，不能承载子级。
  if (!dropToGap && dropMenu.type === 3) return null

  const detached = detachMenu(tree, dragId)
  if (!detached.menu) return null

  const mode: DropMode = dropToGap ? (dropPosition < 0 ? 'before' : 'after') : 'inside'
  const inserted = insertMenu(detached.nodes, dropId, { ...detached.menu }, mode)
  if (!inserted) return null

  const withParent = setMenuParent(inserted.nodes, dragId, inserted.parentId)
  return normalizeOrder(tree, withParent)
}
