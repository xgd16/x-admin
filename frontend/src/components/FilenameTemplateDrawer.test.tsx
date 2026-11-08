/**
 * 导出文件名配置抽屉测试：加载、拖拽排序、片段增删与保存。
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { App as AntdApp } from 'antd'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '../api/client'
import type { FilenameTemplate } from '../api/types'
import { FilenameTemplateDrawer } from './FilenameTemplateDrawer'

vi.mock('../api/client', () => ({
  api: { get: vi.fn(), put: vi.fn() },
}))

const defaultTemplate: FilenameTemplate = {
  segments: [{ type: 'module' }, { type: 'time', format: '20060102150405' }],
}

function renderDrawer() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <AntdApp>
        <FilenameTemplateDrawer open onClose={() => undefined} />
      </AntdApp>
    </QueryClientProvider>,
  )
}

/** 等待模板数据加载完成：预览从占位变为按模板拼接。 */
async function waitLoaded() {
  await screen.findByText(/^预览：users_\d{14}\.xlsx$/)
}

beforeEach(() => {
  vi.mocked(api.get).mockResolvedValue(defaultTemplate)
  vi.mocked(api.put).mockResolvedValue(defaultTemplate)
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('FilenameTemplateDrawer', () => {
  it('打开后加载并按顺序渲染片段与预览', async () => {
    renderDrawer()
    await waitLoaded()
    const handles = document.querySelectorAll('.ant-drawer-section [draggable="true"]')
    expect(handles).toHaveLength(2)
  })

  it('拖拽手柄可以交换片段顺序并标记 dirty', async () => {
    renderDrawer()
    await waitLoaded()
    const handles = document.querySelectorAll('.ant-drawer-section [draggable="true"]')
    const secondRow = handles[1]?.closest('div')
    expect(secondRow).toBeTruthy()

    fireEvent.dragStart(handles[0])
    fireEvent.drop(secondRow!)

    await waitFor(() => expect((screen.getByRole('button', { name: /保\s*存/ }) as HTMLButtonElement).disabled).toBe(false))
    // 顺序交换后预览以时间开头。
    expect(screen.getByText(/^预览：\d{14}_users/)).toBeTruthy()
  })

  it('添加自定义文本片段并保存时提交完整模板', async () => {
    renderDrawer()
    await waitLoaded()
    fireEvent.click(screen.getByRole('button', { name: /自定义文本/ }))
    const input = await screen.findByDisplayValue('导出')
    fireEvent.change(input, { target: { value: '审计' } })

    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }))
    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith('/export-tasks/filename-template', {
        segments: [
          { type: 'module' },
          { type: 'time', format: '20060102150405' },
          { type: 'text', value: '审计' },
        ],
      }),
    )
  })

  it('标题与模块片段唯一，重复添加按钮禁用', async () => {
    renderDrawer()
    await waitLoaded()
    expect((screen.getByRole('button', { name: /模块标识/ }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: /任务标题/ }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('恢复默认重置为模块+时间并允许保存', async () => {
    vi.mocked(api.get).mockResolvedValue({ segments: [{ type: 'title' }] })
    renderDrawer()
    await screen.findByText(/^预览：用户列表\.xlsx$/)

    fireEvent.click(screen.getByRole('button', { name: '恢复默认' }))
    await screen.findByText(/^预览：users_\d{14}\.xlsx$/)
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }))
    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith('/export-tasks/filename-template', {
        segments: [{ type: 'module' }, { type: 'time', format: '20060102150405' }],
      }),
    )
  })

  it('只剩一个片段时删除按钮禁用', async () => {
    vi.mocked(api.get).mockResolvedValue({ segments: [{ type: 'module' }] })
    renderDrawer()
    await screen.findByText(/^预览：users\.xlsx$/)
    await waitFor(() => {
      const deleteButton = document.querySelector('button[title="删除片段"]') as HTMLButtonElement | null
      expect(deleteButton).toBeTruthy()
      expect(deleteButton!.disabled).toBe(true)
    })
  })
})
