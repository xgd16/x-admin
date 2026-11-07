/**
 * 表格列工具：同时供 antd Table 与移动端卡片列表使用。
 */
import type { Key, ReactNode } from 'react'
import type { TableColumnType, TableColumnsType } from 'antd'

/** 为列生成稳定 key：优先显式 key，其次 dataIndex，最后退回下标。 */
export function tableColumnKey(column: { key?: Key; dataIndex?: unknown }, index: number): string {
  return String(column.key ?? column.dataIndex ?? index)
}

/** 把列分组（children）拍平成叶子列，移动端卡片无法展示多级表头。 */
export function flattenTableColumns<T extends object>(columns: TableColumnsType<T>): TableColumnType<T>[] {
  const result: TableColumnType<T>[] = []
  for (const column of columns) {
    if ('children' in column && Array.isArray(column.children)) {
      result.push(...flattenTableColumns(column.children))
    } else {
      result.push(column as TableColumnType<T>)
    }
  }
  return result
}

/** 提取可读列名，复杂表头（ReactNode）不参与移动端标签展示。 */
export function tableColumnTitle<T extends object>(column: TableColumnType<T>): string {
  return typeof column.title === 'string' ? column.title : ''
}

/** 从 dataIndex 中取值，兼容 antd 支持的数组路径。 */
export function readDataIndex(record: unknown, dataIndex: unknown): unknown {
  if (Array.isArray(dataIndex)) {
    return dataIndex.reduce<unknown>((current, key) => {
      if (current === null || current === undefined) return undefined
      return (current as Record<string, unknown>)[String(key)]
    }, record)
  }
  if (typeof dataIndex === 'string' || typeof dataIndex === 'number') {
    if (record === null || record === undefined) return undefined
    return (record as Record<string, unknown>)[String(dataIndex)]
  }
  return undefined
}

/** 按 antd 的 render 规则渲染单元格。 */
export function renderTableColumn<T extends object>(
  column: TableColumnType<T>,
  record: T,
  index: number,
): ReactNode {
  const value = readDataIndex(record, column.dataIndex)
  if (typeof column.render === 'function') {
    return column.render(value, record, index) as ReactNode
  }
  return value as ReactNode
}

/** 操作列：表头为“操作”、key 为 actions，或固定在右侧。 */
export function isTableActionColumn<T extends object>(column: TableColumnType<T>): boolean {
  const key = column.key === undefined || column.key === null ? '' : String(column.key)
  return key === 'actions' || tableColumnTitle(column) === '操作' || column.fixed === 'right'
}

/** 空值统一展示为破折号，避免移动端卡片出现空白行。 */
export function isEmptyTableValue(value: ReactNode): boolean {
  return value === null || value === undefined || value === ''
}

/** 长文本 / 多行文本 / 省略列在卡片中独占整行。 */
export function isWideTableColumn<T extends object>(column: TableColumnType<T>, raw: unknown): boolean {
  if (column.ellipsis) return true
  if (typeof raw === 'string') return raw.length > 26 || raw.includes('\n')
  return false
}

/** 判断 ReactNode 是否真正包含可见内容（空 div 不算）。 */
export function hasVisibleNode(node: ReactNode): boolean {
  if (isEmptyTableValue(node) || node === false) return false
  if (Array.isArray(node)) return node.some(hasVisibleNode)
  if (typeof node === 'object' && node !== null && 'props' in node) {
    const children = (node.props as { children?: ReactNode }).children
    if (children !== undefined) return hasVisibleNode(children)
  }
  return true
}
