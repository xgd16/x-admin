import type { ReactElement } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FileSelect } from './FileSelect'

vi.mock('../stores/auth', () => ({
  useAuth: (selector: (state: { can: () => boolean }) => unknown) => selector({ can: () => false }),
}))

function renderSelect(element: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{element}</QueryClientProvider>)
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('FileSelect', () => {
  it('renders a stored url and clears it through the remove action', () => {
    const onChange = vi.fn()
    renderSelect(<FileSelect value="/uploads/2026/11/avatar.png" types="image" onChange={onChange} />)

    expect(screen.getByRole('img', { name: 'avatar.png' })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '移除 avatar.png' }))
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('keeps arrays for multiple values', () => {
    const onChange = vi.fn()
    renderSelect(<FileSelect value={['/uploads/a.png', '/uploads/b.pdf']} types={['image', 'document']} multiple onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: '移除 a.png' }))
    expect(onChange).toHaveBeenCalledWith(['/uploads/b.pdf'])
  })

  it('shows an add tile when empty', () => {
    renderSelect(<FileSelect value="" types="image" />)
    expect(screen.getByRole('button', { name: /选择图片/ })).toBeTruthy()
  })
})
