import { useEffect, useMemo, useState, type Key, type ReactNode } from 'react'
import { Checkbox, Dropdown, Popover, Table } from 'antd'
import type { TableColumnsType, TableProps } from 'antd'
import { Icon } from './Icon'
import { useUi } from '../stores/ui'

interface DataTableProps<T> {
  columns: TableColumnsType<T>
  data: T[]
  rowKey: string | ((row: T) => string | number)
  storageKey: string
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
  onPageChange?: (page: number, pageSize: number) => void
  toolbar?: ReactNode
  filters?: ReactNode
  onRefresh?: () => void
  selection?: TableProps<T>['rowSelection']
  scroll?: TableProps<T>['scroll']
  expandable?: TableProps<T>['expandable']
  onRow?: TableProps<T>['onRow']
  emptyText?: ReactNode
}

function columnKey(column: { key?: Key; dataIndex?: unknown }, index: number): string {
  return String(column.key ?? column.dataIndex ?? index)
}

export function DataTable<T extends object>({
  columns,
  data,
  rowKey,
  storageKey,
  loading,
  total = 0,
  page = 1,
  pageSize = 20,
  onPageChange,
  toolbar,
  filters,
  onRefresh,
  selection,
  scroll,
  expandable,
  onRow,
  emptyText,
}: DataTableProps<T>) {
  const density = useUi((state) => state.density)
  const setDensity = useUi((state) => state.setDensity)
  const persisted = useUi((state) => state.columns[storageKey])
  const setColumns = useUi((state) => state.setColumns)
  const allKeys = useMemo(
    () => columns.map((column, index) => columnKey(column as TableColumnsType<never>[number], index)),
    [columns],
  )
  const [hidden, setHidden] = useState<string[]>([])

  useEffect(() => {
    setHidden((current) => current.filter((key) => allKeys.includes(key)))
  }, [allKeys])

  const visibleKeys = persisted?.length ? persisted.filter((key) => allKeys.includes(key)) : allKeys
  const visibleColumns = columns.filter((column, index) => visibleKeys.includes(columnKey(column as TableColumnsType<never>[number], index)))

  const toggle = (key: string, checked: boolean) => {
    const next = checked
      ? Array.from(new Set([...visibleKeys, key]))
      : visibleKeys.filter((item) => item !== key)
    setColumns(storageKey, next)
    setHidden(hidden.filter((item) => item !== key))
  }

  return (
    <div className="data-panel glass">
      {filters && <div className="filter-bar">{filters}</div>}
      <div className="table-toolbar">
        <div className="left">{toolbar}</div>
        <div className="right">
          <Popover
            trigger="click"
            placement="bottomRight"
            content={
              <div style={{ display: 'grid', gap: 6, minWidth: 150 }}>
                {columns.map((column, index) => {
                  const key = columnKey(column as TableColumnsType<never>[number], index)
                  const title = typeof column.title === 'string' ? column.title : key
                  return (
                    <Checkbox key={key} checked={visibleKeys.includes(key)} onChange={(event) => toggle(key, event.target.checked)}>
                      {title}
                    </Checkbox>
                  )
                })}
              </div>
            }
          >
            <button className="icon-btn" title="列设置" type="button">
              <Icon name="columns" size={15} />
            </button>
          </Popover>
          <Dropdown
            trigger={['click']}
            menu={{
              selectedKeys: [density],
              items: [
                { key: 'small', label: '紧凑' },
                { key: 'middle', label: '默认' },
                { key: 'large', label: '宽松' },
              ],
              onClick: ({ key }) => setDensity(key as 'small' | 'middle' | 'large'),
            }}
          >
            <button className="icon-btn" title="表格密度" type="button">
              <Icon name="list" size={15} />
            </button>
          </Dropdown>
          {onRefresh && (
            <button className="icon-btn" title="刷新" type="button" onClick={onRefresh}>
              <Icon name="refresh" size={15} />
            </button>
          )}
        </div>
      </div>
      <Table<T>
        rowKey={rowKey}
        columns={visibleColumns}
        dataSource={data}
        loading={loading}
        size={density}
        rowSelection={selection}
        expandable={expandable}
        onRow={onRow}
        scroll={scroll ?? { x: 'max-content' }}
        locale={{ emptyText: emptyText ?? '暂无数据' }}
        pagination={
          total > 0
            ? {
                current: page,
                pageSize,
                total,
                showSizeChanger: true,
                pageSizeOptions: [10, 20, 50, 100],
                showTotal: (count) => `共 ${count} 条`,
                onChange: onPageChange,
              }
            : false
        }
      />
    </div>
  )
}
