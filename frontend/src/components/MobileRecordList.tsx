/**
 * 移动端密集数据列表：把表格行转成卡片，供 DataTable 与自定义表格复用。
 *
 * 设计目标：
 * - 同一份 columns 同时驱动桌面表格与手机卡片，页面无需维护两套字段。
 * - 首列作为卡片主标题，其余列作为标签/值，操作列固定在卡片底部。
 * - 支持行选择、树形展开、加载态、空状态与分页。
 */
import { useMemo, type Key, type ReactNode } from 'react'
import { Checkbox, Empty, Pagination, Spin } from 'antd'
import type { TableColumnType, TableColumnsType, TableProps } from 'antd'
import { Icon } from './Icon'
import { ListSkeleton, LoadingIndicator } from './Loading'
import {
  flattenTableColumns,
  hasVisibleNode,
  isEmptyTableValue,
  isTableActionColumn,
  isWideTableColumn,
  readDataIndex,
  renderTableColumn,
  tableColumnKey,
  tableColumnTitle,
} from '../utils/table'

interface MobileRecordListPagination {
  current: number
  pageSize: number
  total: number
  onChange: (page: number, pageSize: number) => void
}

export interface MobileRecordListProps<T extends object> {
  columns: TableColumnsType<T>
  data: T[]
  rowKey: string | ((row: T) => Key)
  loading?: boolean
  emptyText?: ReactNode
  selection?: TableProps<T>['rowSelection']
  onRow?: TableProps<T>['onRow']
  expandable?: TableProps<T>['expandable']
  pagination?: MobileRecordListPagination
  /** 可选覆盖：指定主标题列、隐藏列或强制操作列。 */
  mobile?: {
    titleKey?: string
    hiddenKeys?: string[]
    actionKeys?: string[]
  }
}

interface KeyedColumn<T> {
  column: TableColumnType<T>
  key: string
}

/** 解析行 key，保持与 antd Table 的 rowKey 行为一致。 */
function resolveRowKey<T extends object>(rowKey: string | ((row: T) => Key), record: T): Key {
  if (typeof rowKey === 'function') return rowKey(record)
  return (record as Record<string, unknown>)[rowKey] as Key
}

/** 判断某个 key 是否在选中集合里。 */
function isKeySelected(selectedKeys: readonly Key[], key: Key): boolean {
  return selectedKeys.some((item) => item === key)
}

export function MobileRecordList<T extends object>({
  columns,
  data,
  rowKey,
  loading,
  emptyText,
  selection,
  onRow,
  expandable,
  pagination,
  mobile,
}: MobileRecordListProps<T>) {
  const flatColumns = useMemo(() => flattenTableColumns(columns), [columns])
  const hiddenKeys = useMemo(() => new Set(mobile?.hiddenKeys ?? []), [mobile?.hiddenKeys])
  const actionKeys = useMemo(() => new Set(mobile?.actionKeys ?? []), [mobile?.actionKeys])

  const keyedColumns = useMemo<KeyedColumn<T>[]>(
    () => flatColumns.map((column, index) => ({ column, key: tableColumnKey(column, index) })),
    [flatColumns],
  )

  // 主标题列：默认取第一列；操作列永远不参与标题与字段。
  const titleEntry = useMemo(() => {
    if (mobile?.titleKey) {
      return keyedColumns.find((entry) => entry.key === mobile.titleKey)
    }
    return keyedColumns.find((entry) => !hiddenKeys.has(entry.key) && !isTableActionColumn(entry.column))
  }, [keyedColumns, hiddenKeys, mobile?.titleKey])

  const fieldEntries = useMemo(
    () =>
      keyedColumns.filter(
        (entry) =>
          entry.key !== titleEntry?.key &&
          !hiddenKeys.has(entry.key) &&
          !isTableActionColumn(entry.column),
      ),
    [keyedColumns, titleEntry?.key, hiddenKeys],
  )

  const actionEntries = useMemo(
    () =>
      keyedColumns.filter(
        (entry) => !hiddenKeys.has(entry.key) && (actionKeys.has(entry.key) || isTableActionColumn(entry.column)),
      ),
    [keyedColumns, actionKeys, hiddenKeys],
  )

  const selectedKeys = selection?.selectedRowKeys ?? []
  const childrenColumnName = expandable?.childrenColumnName ?? 'children'
  const expandedKeys = expandable?.expandedRowKeys

  // 当前页可选项：树形表格只把顶层行纳入“全选本页”。
  const selectableRows = useMemo(
    () => data.filter((record) => !selection?.getCheckboxProps?.(record)?.disabled),
    [data, selection],
  )
  const allSelected = selectableRows.length > 0 && selectableRows.every((record) => isKeySelected(selectedKeys, resolveRowKey(rowKey, record)))
  const partiallySelected = !allSelected && selectableRows.some((record) => isKeySelected(selectedKeys, resolveRowKey(rowKey, record)))

  const toggleAll = (checked: boolean) => {
    if (!selection?.onChange) return
    if (checked) {
      const next = [...selectedKeys]
      for (const record of selectableRows) {
        const key = resolveRowKey(rowKey, record)
        if (!isKeySelected(next, key)) next.push(key)
      }
      selection.onChange(next, selectableRows, { type: 'all' })
      return
    }
    const pageKeys = new Set(selectableRows.map((record) => resolveRowKey(rowKey, record)))
    selection.onChange(
      selectedKeys.filter((key) => !pageKeys.has(key)),
      [],
      { type: 'none' },
    )
  }

  const toggleRow = (record: T, checked: boolean) => {
    if (!selection?.onChange) return
    const key = resolveRowKey(rowKey, record)
    const next = checked
      ? isKeySelected(selectedKeys, key)
        ? selectedKeys
        : [...selectedKeys, key]
      : selectedKeys.filter((item) => item !== key)
    const rows = data.filter((item) => isKeySelected(next, resolveRowKey(rowKey, item)))
    selection.onChange(next, rows, { type: 'single' })
  }

  const toggleExpand = (record: T, key: Key) => {
    if (!expandable) return
    const current = expandedKeys ?? []
    const expanded = isKeySelected(current, key)
    const next = expanded ? current.filter((item) => item !== key) : [...current, key]
    expandable.onExpandedRowsChange?.(next)
    expandable.onExpand?.(!expanded, record)
  }

  const renderCard = (record: T, index: number, depth = 0): ReactNode => {
    const key = resolveRowKey(rowKey, record)
    const selected = isKeySelected(selectedKeys, key)
    const checkboxProps = selection?.getCheckboxProps?.(record) ?? {}
    const rowProps = onRow?.(record, index) ?? {}
    const children = (record as Record<string, unknown>)[childrenColumnName]
    const hasChildren = Array.isArray(children) && children.length > 0
    const rowExpandable = hasChildren && (expandable?.rowExpandable?.(record) ?? true)
    const expanded = isKeySelected(expandedKeys ?? [], key)

    const fields = fieldEntries.map(({ column, key: columnId }) => {
      const raw = readDataIndex(record, column.dataIndex)
      const value = renderTableColumn(column, record, index)
      if (isEmptyTableValue(value)) return null
      return (
        <div
          key={columnId}
          className={`m-card-field ${isWideTableColumn(column, raw) ? 'wide' : ''}`}
        >
          <span className="m-card-label">{tableColumnTitle(column) || columnId}</span>
          <div className="m-card-value">{value}</div>
        </div>
      )
    })

    const actions = actionEntries
      .map(({ column, key: columnId }) => {
        const node = renderTableColumn(column, record, index)
        if (!hasVisibleNode(node)) return null
        return (
          <div className="m-card-action" key={columnId}>
            {node}
          </div>
        )
      })
      .filter(Boolean)

    return (
      <div key={String(key)} className="m-card-wrap" style={depth > 0 ? { paddingLeft: Math.min(depth, 4) * 14 } : undefined}>
        <article
          className={`m-card ${selected ? 'selected' : ''} ${rowProps.className ?? ''}`}
          style={rowProps.style}
          data-clickable={rowProps.onClick ? 'true' : undefined}
          onClick={rowProps.onClick}
        >
          <div className="m-card-head">
            {selection && (
              <span className="m-card-check" onClick={(event) => event.stopPropagation()}>
                <Checkbox
                  checked={selected}
                  disabled={checkboxProps.disabled}
                  aria-label={`选择第 ${index + 1} 行`}
                  onChange={(event) => toggleRow(record, event.target.checked)}
                />
              </span>
            )}
            <div className="m-card-primary">
              {titleEntry ? renderTableColumn(titleEntry.column, record, index) : null}
            </div>
            {expandable && rowExpandable && (
              <button
                type="button"
                className={`m-card-expand ${expanded ? 'open' : ''}`}
                aria-label={expanded ? '收起子项' : '展开子项'}
                aria-expanded={expanded}
                onClick={(event) => {
                  event.stopPropagation()
                  toggleExpand(record, key)
                }}
              >
                <Icon name="chevronDown" size={14} />
              </button>
            )}
            {!(expandable && rowExpandable) && rowProps.onClick && (
              <span className="m-card-chevron" aria-hidden="true">
                <Icon name="chevronRight" size={14} />
              </span>
            )}
          </div>
          {fields.some(Boolean) && <div className="m-card-fields">{fields}</div>}
          {actions.length > 0 && (
            <div className="m-card-actions" onClick={(event) => event.stopPropagation()}>
              {actions}
            </div>
          )}
        </article>
        {expandable && rowExpandable && expanded && Array.isArray(children)
          ? (children as T[]).map((child, childIndex) => renderCard(child, childIndex, depth + 1))
          : null}
      </div>
    )
  }

  return (
    <Spin
      spinning={!!loading && data.length > 0}
      delay={160}
      indicator={<LoadingIndicator />}
      description={data.length > 0 && loading ? '正在同步数据' : undefined}
      classNames={{ root: 'm-list-spin' }}
    >
      <div className="m-list">
        {!!loading && data.length === 0 ? (
          <ListSkeleton rows={4} />
        ) : (
          <>
            {selection && data.length > 0 && (
              <div className="m-list-select">
                <Checkbox
                  checked={allSelected}
                  indeterminate={partiallySelected}
                  onChange={(event) => toggleAll(event.target.checked)}
                >
                  全选本页
                </Checkbox>
                <span>{selectedKeys.length > 0 ? `已选 ${selectedKeys.length} 项` : '未选择'}</span>
              </div>
            )}
            {data.length === 0 && !loading && (
              <div className="m-list-empty">
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText ?? '暂无数据'} />
              </div>
            )}
            {data.map((record, index) => renderCard(record, index))}
          </>
        )}
      </div>
      {pagination && pagination.total > 0 && (
        <div className="m-list-pagination">
          <span>共 {pagination.total} 条</span>
          <Pagination
            simple
            size="small"
            current={pagination.current}
            pageSize={pagination.pageSize}
            total={pagination.total}
            onChange={pagination.onChange}
          />
        </div>
      )}
    </Spin>
  )
}
