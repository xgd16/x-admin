/**
 * 导出记录页测试：列表渲染、按状态轮询、下载与取消调用。
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { App as AntdApp } from 'antd'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '../api/client'
import { Exports } from './Exports'
import type { ExportTask, PageResult } from '../api/types'

vi.mock('../api/client', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    del: vi.fn(),
    download: vi.fn(),
  },
}))

// 全部放行权限码，只验证页面交互本身。
vi.mock('../stores/auth', () => ({
  useAuth: (selector: (state: { can: () => boolean }) => unknown) => selector({ can: () => true }),
}))

function makeTask(overrides: Partial<ExportTask>): ExportTask {
  return {
    id: 1,
    user_id: 1,
    module: 'users',
    title: '用户列表',
    status: 2,
    total: 10,
    processed: 10,
    file_name: 'users.xlsx',
    file_size: 4096,
    finished_at: '2026-11-08T10:00:00+08:00',
    created_at: '2026-11-08T09:59:00+08:00',
    ...overrides,
  }
}

const page = (list: ExportTask[]): PageResult<ExportTask> => ({
  list,
  total: list.length,
  page: 1,
  page_size: 20,
})

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <AntdApp>
        <MemoryRouter>
          <Exports />
        </MemoryRouter>
      </AntdApp>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.mocked(api.get).mockResolvedValue(page([]))
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('Exports', () => {
  it('渲染任务列表与状态标签', async () => {
    vi.mocked(api.get).mockResolvedValue(
      page([
        makeTask({ status: 2, title: '用户列表' }),
        makeTask({ id: 2, status: 3, title: '操作日志', module: 'operation_logs', error: '读取数据失败' }),
      ]),
    )
    renderPage()

    expect(await screen.findByText('用户列表')).toBeTruthy()
    expect(await screen.findByText('已完成')).toBeTruthy()
    expect(await screen.findByText('失败')).toBeTruthy()
  })

  it('运行中的任务显示进度条，已完成显示总行数', async () => {
    vi.mocked(api.get).mockResolvedValue(
      page([
        makeTask({ status: 1, total: 100, processed: 40 }),
        makeTask({ id: 2, status: 2, processed: 30 }),
      ]),
    )
    renderPage()

    expect(await screen.findByRole('progressbar')).toBeTruthy()
    expect(await screen.findByText('共 30 行')).toBeTruthy()
  })

  it('存在未完成任务时每 2 秒轮询，全部结束后停止', async () => {
    vi.useFakeTimers()
    const running = page([makeTask({ status: 1 })])
    const done = page([makeTask({ status: 2 })])
    vi.mocked(api.get).mockResolvedValueOnce(running).mockResolvedValueOnce(running).mockResolvedValue(done)

    renderPage()
    await vi.advanceTimersByTimeAsync(0)
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(screen.getByText('生成中')).toBeTruthy()

    await vi.advanceTimersByTimeAsync(2000)
    expect(api.get).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(2000)
    expect(api.get).toHaveBeenCalledTimes(3)

    // 最后一轮拿到已完成列表，轮询停止。
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get).toHaveBeenCalledTimes(3)
  })

  it('完成任务触发文件下载', async () => {
    const createObjectURL = vi.fn(() => 'blob:mock')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', { value: createObjectURL, configurable: true, writable: true })
    Object.defineProperty(URL, 'revokeObjectURL', { value: revokeObjectURL, configurable: true, writable: true })

    vi.mocked(api.get).mockResolvedValue(page([makeTask({ status: 2 })]))
    vi.mocked(api.download).mockResolvedValue(new Blob(['PK'], { type: 'application/zip' }))

    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: /下载/ }))
    await waitFor(() => expect(api.download).toHaveBeenCalledWith('/export-tasks/1/download'))
  })

  it('运行中的任务可取消并调用取消接口', async () => {
    vi.mocked(api.get).mockResolvedValue(page([makeTask({ status: 1 })]))
    vi.mocked(api.post).mockResolvedValue(null)

    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: '取消' }))
    await waitFor(() => expect(api.post).toHaveBeenCalledWith('/export-tasks/1/cancel'))
  })

  it('已完成任务需二次确认后删除', async () => {
    vi.mocked(api.get).mockResolvedValue(page([makeTask({ status: 2 })]))
    vi.mocked(api.del).mockResolvedValue(null)

    renderPage()
    // 列表行内的删除按钮只有无任务时一个；点击后出现确认弹窗的“删除”。
    fireEvent.click(await screen.findByRole('button', { name: /删除/ }))
    const confirmButtons = await screen.findAllByRole('button', { name: /删\s*除/ })
    fireEvent.click(confirmButtons[confirmButtons.length - 1])
    await waitFor(() => expect(api.del).toHaveBeenCalledWith('/export-tasks/1'))
  })
})
