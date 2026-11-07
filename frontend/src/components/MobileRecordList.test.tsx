/**
 * 移动端密集列表：字段映射、行选择与操作列的回归测试。
 */
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { TableColumnsType } from 'antd'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MobileRecordList } from './MobileRecordList'

interface Row {
  id: number
  name: string
  status: string
  children?: Row[]
}

const onDelete = vi.fn()

const columns: TableColumnsType<Row> = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '状态', dataIndex: 'status', key: 'status' },
  {
    title: '操作',
    key: 'actions',
    render: (_, row) => <button onClick={() => onDelete(row.id)}>删除</button>,
  },
]

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('MobileRecordList', () => {
  it('把首列作为标题，其余列输出为标签/值', () => {
    render(
      <MobileRecordList<Row>
        columns={columns}
        data={[{ id: 1, name: '管理员', status: '正常' }]}
        rowKey="id"
      />,
    )

    expect(screen.getByText('管理员')).toBeTruthy()
    expect(screen.getByText('状态')).toBeTruthy()
    expect(screen.getByText('正常')).toBeTruthy()
  })

  it('操作列固定在卡片底部，并保留原有点击行为', () => {
    render(
      <MobileRecordList<Row>
        columns={columns}
        data={[{ id: 7, name: '运营', status: '停用' }]}
        rowKey="id"
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '删除' }))
    expect(onDelete).toHaveBeenCalledWith(7)
  })

  it('行选择使用与表格一致的 selectedRowKeys', () => {
    const onChange = vi.fn()
    const row = { id: 3, name: '审计员', status: '正常' }

    render(
      <MobileRecordList<Row>
        columns={columns}
        data={[row]}
        rowKey="id"
        selection={{ selectedRowKeys: [], onChange }}
      />,
    )

    fireEvent.click(screen.getByLabelText('选择第 1 行'))
    expect(onChange).toHaveBeenCalledWith([3], [row], { type: 'single' })
  })

  it('当前页为空但总数大于 0 时仍保留分页入口', () => {
    render(
      <MobileRecordList<Row>
        columns={columns}
        data={[]}
        rowKey="id"
        pagination={{ current: 2, pageSize: 10, total: 11, onChange: vi.fn() }}
      />,
    )

    expect(screen.getByText('暂无数据')).toBeTruthy()
    expect(screen.getByText('共 11 条')).toBeTruthy()
  })

  it('树形数据按 expandedRowKeys 展开子项', () => {
    const tree: Row[] = [
      { id: 1, name: '技术部', status: '正常', children: [{ id: 2, name: '前端组', status: '正常' }] },
    ]
    const onExpandedRowsChange = vi.fn()

    const { rerender } = render(
      <MobileRecordList<Row>
        columns={columns}
        data={tree}
        rowKey="id"
        expandable={{ expandedRowKeys: [], onExpandedRowsChange }}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '展开子项' }))
    expect(onExpandedRowsChange).toHaveBeenCalledWith([1])
    expect(screen.queryByText('前端组')).toBeNull()

    rerender(
      <MobileRecordList<Row>
        columns={columns}
        data={tree}
        rowKey="id"
        expandable={{ expandedRowKeys: [1], onExpandedRowsChange }}
      />,
    )
    expect(screen.getByText('前端组')).toBeTruthy()
  })
})
