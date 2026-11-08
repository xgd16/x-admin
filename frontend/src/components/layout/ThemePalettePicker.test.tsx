/**
 * 主题色板选择器的渲染与切换行为。
 */
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_PALETTE, PALETTES } from '../../config/themes'
import { useUi } from '../../stores/ui'
import { ThemePalettePicker } from './ThemePalettePicker'

function reset() {
  useUi.setState({ palette: DEFAULT_PALETTE })
}

describe('ThemePalettePicker', () => {
  beforeEach(reset)
  afterEach(() => {
    cleanup()
    reset()
  })

  it('renders every palette and marks the current one', () => {
    render(<ThemePalettePicker />)

    expect(screen.getAllByRole('radio')).toHaveLength(PALETTES.length)
    expect(screen.getByRole('radio', { name: '珊瑚橙' }).getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('radio', { name: '海洋蓝' }).getAttribute('aria-checked')).toBe('false')
  })

  it('switches the palette when a swatch is clicked', () => {
    render(<ThemePalettePicker />)

    fireEvent.click(screen.getByRole('radio', { name: '海洋蓝' }))

    expect(useUi.getState().palette).toBe('ocean')
    expect(screen.getByRole('radio', { name: '海洋蓝' }).getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('radio', { name: '珊瑚橙' }).getAttribute('aria-checked')).toBe('false')
  })
})
