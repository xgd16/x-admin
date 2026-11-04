/**
 * 文件选择弹窗的交互测试。
 */
import type { ComponentProps } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../api/client'
import type { FileItem } from '../api/types'
import { FilePicker } from './FilePicker'

// 隔离真实请求；默认账号没有上传权限，上传按钮不会出现。
vi.mock('../api/client', () => ({
  api: { get: vi.fn(), upload: vi.fn() },
}))

vi.mock('../stores/auth', () => ({
  useAuth: (selector: (state: { can: () => boolean }) => unknown) => selector({ can: () => false }),
}))

const files: FileItem[] = [
  {
    id: 1,
    name: 'cover.png',
    key: '2026/11/cover.png',
    url: '/uploads/2026/11/cover.png',
    mime: 'image/png',
    ext: 'png',
    size: 2048,
    uploader_id: 1,
    created_at: '2026-11-03T10:00:00Z',
  },
  {
    id: 2,
    name: 'clip.mp4',
    key: '2026/11/clip.mp4',
    url: '/uploads/2026/11/clip.mp4',
    mime: 'video/mp4',
    ext: 'mp4',
    size: 4096,
    uploader_id: 1,
    created_at: '2026-11-03T10:00:00Z',
  },
]

function fileCard(name: string): HTMLElement {
  const card = [...document.querySelectorAll<HTMLElement>('.file-card')].find((item) => item.title === name)
  if (!card) throw new Error(`file card not found: ${name}`)
  return card
}

function renderPicker(props: Partial<ComponentProps<typeof FilePicker>> = {}) {
  const onConfirm = vi.fn()
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const result = render(
    <QueryClientProvider client={queryClient}>
      <FilePicker open types="image" onCancel={vi.fn()} onConfirm={onConfirm} {...props} />
    </QueryClientProvider>,
  )
  return { ...result, onConfirm }
}

// 每个用例都返回同一份文件库数据。
beforeEach(() => {
  vi.mocked(api.get).mockResolvedValue({ list: files, total: files.length, page: 1, page_size: 24 })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('FilePicker', () => {
  it('loads only the requested kinds and returns a single selection', async () => {
    const { onConfirm } = renderPicker()
    await screen.findByText('cover.png')

    expect(api.get).toHaveBeenCalledWith('/files', { page: 1, page_size: 24, keyword: '', kind: 'image' })

    fireEvent.click(fileCard('cover.png'))
    expect(fileCard('cover.png').getAttribute('aria-pressed')).toBe('true')

    fireEvent.click(screen.getByRole('button', { name: '使用所选文件' }))
    expect(onConfirm).toHaveBeenCalledWith([files[0]])
  })

  it('keeps selection across cards and reports the count when multiple', async () => {
    const { onConfirm } = renderPicker({ types: ['image', 'video'], multiple: true })
    await screen.findByText('cover.png')

    fireEvent.click(fileCard('cover.png'))
    fireEvent.click(fileCard('clip.mp4'))

    expect(screen.getByRole('button', { name: '使用所选 2 项' })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '使用所选 2 项' }))
    expect(onConfirm).toHaveBeenCalledWith(files)
  })

  it('enforces a maximum when one is supplied', async () => {
    renderPicker({ types: ['image', 'video'], multiple: true, limit: 1 })
    await screen.findByText('cover.png')

    fireEvent.click(fileCard('cover.png'))
    fireEvent.click(fileCard('clip.mp4'))

    await waitFor(() => {
      expect(screen.getByText('最多选择 1 个文件')).toBeTruthy()
    })
    expect(screen.getByRole('button', { name: '使用所选 1 项' })).toBeTruthy()
  })
})
