/**
 * 列表页通用表格：列显示偏好、密度、刷新与分页。
 * 窄屏或触屏窄窗口自动切换为卡片列表，页面无需维护两套渲染逻辑。
 */
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Checkbox, Dropdown, Popover, Table } from 'antd'
import type { TableColumnsType, TableProps } from 'antd'
import { Icon } from './Icon'
import { LoadingIndicator, TableSkeleton } from './Loading'
import { MobileRecordList } from './MobileRecordList'
import { useIsCompactViewport } from '../hooks/useMediaQuery'
import { tableColumnKey } from '../utils/table'
import { useUi } from '../stores/ui'

interface DataTableProps<T extends object> {
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
  /** 移动端卡片覆盖项；默认首列为主标题，操作列固定在卡片底部。 */
  mobile?: {
    titleKey?: string
    hiddenKeys?: string[]
    actionKeys?: string[]
  }
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
  mobile,
}: DataTableProps<T>) {
  const density = useUi((state) => state.density)
  const setDensity = useUi((state) => state.setDensity)
  // 列显示偏好按 storageKey 持久化，同一个表格刷新后仍保持用户选择。
  const persisted = useUi((state) => state.columns[storageKey])
  const setColumns = useUi((state) => state.setColumns)
  const [hidden, setHidden] = useState<string[]>([])
  const [filtersOpen, setFiltersOpen] = useState(false)
  const compact = useIsCompactViewport()

  const allKeys = useMemo(
    () => columns.map((column, index) => tableColumnKey(column as TableColumnsType<never>[number], index)),
    [columns],
  )

  // 列定义变化时清理已不存在的隐藏项，避免偏好里残留脏 key。
  useEffect(() => {
    setHidden((current) => current.filter((key) => allKeys.includes(key)))
  }, [allKeys])

  const visibleKeys = persisted?.length ? persisted.filter((key) => allKeys.includes(key)) : allKeys
  const visibleColumns = columns.filter((column, index) => visibleKeys.includes(tableColumnKey(column as TableColumnsType<never>[number], index)))
  // 首屏没有数据时用骨架屏占位；已有数据时刷新只显示轻量遮罩，避免整张表闪白。
  const initialLoading = !!loading && data.length === 0
  const refreshing = !!loading && data.length > 0

  // 勾选/取消列时写回全局偏好，并同步清理本地隐藏集合。
  const toggle = (key: string, checked: boolean) => {
    const next = checked
      ? Array.from(new Set([...visibleKeys, key]))
      : visibleKeys.filter((item) => item !== key)
    setColumns(storageKey, next)
    setHidden(hidden.filter((item) => item !== key))
  }

  return (
    <div className={`data-panel glass ${compact ? 'is-compact' : ''}`} aria-busy={loading || undefined}>
      {filters && (!compact || filtersOpen) && <div className="filter-bar">{filters}</div>}
      <div className="table-toolbar">
        {/* 左侧放业务操作，右侧固定为筛选、列设置、密度和刷新。 */}
        <div className="left">{toolbar}</div>
        <div className="right">
          {compact && filters && (
            <button
              className={`icon-btn ${filtersOpen ? 'on' : ''}`}
              title={filtersOpen ? '收起筛选条件' : '展开筛选条件'}
              aria-label={filtersOpen ? '收起筛选条件' : '展开筛选条件'}
              aria-expanded={filtersOpen}
              type="button"
              onClick={() => setFiltersOpen((open) => !open)}
            >
              <Icon name="sliders" size={15} />
            </button>
          )}
          <Popover
            trigger="click"
            placement="bottomRight"
            content={
              <div style={{ display: 'grid', gap: 6, minWidth: 150 }}>
                {columns.map((column, index) => {
                  const key = tableColumnKey(column as TableColumnsType<never>[number], index)
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
          {!compact && (
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
          )}
          {onRefresh && (
            <button
              className="icon-btn"
              title="刷新"
              type="button"
              aria-busy={loading || undefined}
              onClick={onRefresh}
            >
              <Icon name="refresh" size={15} className={loading ? 'x-spin-icon' : undefined} />
            </button>
          )}
        </div>
      </div>
      {compact ? (
        <MobileRecordList<T>
          columns={visibleColumns}
          data={data}
          rowKey={rowKey}
          loading={loading}
          emptyText={emptyText}
          selection={selection}
          onRow={onRow}
          expandable={expandable}
          mobile={mobile}
          pagination={
            total > 0
              ? {
                  current: page,
                  pageSize,
                  total,
                  onChange: (nextPage, nextPageSize) => onPageChange?.(nextPage, nextPageSize),
                }
              : undefined
          }
        />
      ) : (
        <Table<T>
          rowKey={rowKey}
          columns={visibleColumns}
          dataSource={data}
          loading={
            refreshing
              ? {
                  spinning: true,
                  delay: 160,
                  indicator: <LoadingIndicator />,
                  description: '正在同步数据',
                }
              : false
          }
          size={density}
          rowSelection={selection}
          expandable={expandable}
          onRow={onRow}
          scroll={scroll ?? { x: 'max-content' }}
          locale={{ emptyText: initialLoading ? <TableSkeleton /> : (emptyText ?? '暂无数据') }}
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
      )}
    </div>
  )
}
