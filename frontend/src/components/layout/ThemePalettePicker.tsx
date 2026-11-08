/**
 * 偏好设置里的主题色板选择器：一排色块，点击即时切换。
 */
import { PALETTES } from '../../config/themes'
import { useUi } from '../../stores/ui'
import { Icon } from '../Icon'

export function ThemePalettePicker() {
  const palette = useUi((state) => state.palette)
  const setPalette = useUi((state) => state.setPalette)
  const current = PALETTES.find((preset) => preset.id === palette) ?? PALETTES[0]

  return (
    <div className="settings-themes">
      <div className="settings-themes-head">
        <span className="label">主题色</span>
        <span className="value">{current.label}</span>
      </div>
      <div className="settings-swatches" role="radiogroup" aria-label="主题色">
        {PALETTES.map((preset) => {
          const active = preset.id === palette
          return (
            <button
              key={preset.id}
              type="button"
              role="radio"
              aria-checked={active}
              aria-label={preset.label}
              title={preset.label}
              className={`settings-swatch${active ? ' active' : ''}`}
              style={{ background: `linear-gradient(145deg, ${preset.accent2}, ${preset.accent})` }}
              onClick={() => setPalette(preset.id)}
            >
              {active && <Icon name="check" size={10} />}
            </button>
          )
        })}
      </div>
    </div>
  )
}
