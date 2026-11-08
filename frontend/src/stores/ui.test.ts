/**
 * 界面偏好 store：主题色板的默认值与持久化。
 */
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_PALETTE } from '../config/themes'
import { useUi } from './ui'

function reset() {
  useUi.setState({ theme: 'light', palette: DEFAULT_PALETTE, density: 'middle', watermark: false })
}

describe('useUi palette', () => {
  beforeEach(reset)
  afterEach(reset)

  it('defaults to the coral palette', () => {
    expect(useUi.getState().palette).toBe(DEFAULT_PALETTE)
    expect(useUi.getState().palette).toBe('coral')
  })

  it('switches palette and persists the choice', () => {
    useUi.getState().setPalette('ocean')

    expect(useUi.getState().palette).toBe('ocean')
    const persisted = JSON.parse(localStorage.getItem('x-admin.ui') ?? '{}')
    expect(persisted.state.palette).toBe('ocean')
  })
})
