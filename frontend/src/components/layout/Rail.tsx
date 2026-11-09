/**
 * 左侧模块图标栏：模块切换与主题切换。
 */
import { motion, useReducedMotion } from 'motion/react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useNav } from '../../config/nav'
import { useUi } from '../../stores/ui'
import { SPRING } from '../../utils/motion'
import { BrandMark } from '../BrandMark'
import { Icon } from '../Icon'

export function Rail() {
  const navigate = useNavigate()
  const location = useLocation()
  const theme = useUi((state) => state.theme)
  const toggleTheme = useUi((state) => state.toggleTheme)
  const reduceMotion = useReducedMotion()

  // 后端菜单树决定模块与顺序，只展示至少有一个可访问页面的模块。
  const { items, groups, byPath } = useNav()
  const currentGroup = byPath(location.pathname).group
  const modules = groups
    .map((module) => ({
      ...module,
      first: items.find((item) => item.group === module.group),
    }))
    .filter((module) => module.first)

  return (
    <aside className="rail glass">
      <div className="rail-brand">
        <button className="rail-logo" type="button" onClick={() => navigate(items[0]?.path ?? '/profile')} title="x-admin" aria-label="x-admin 首页">
          <BrandMark size={24} />
        </button>
      </div>
      <span className="rail-sep" aria-hidden="true" />
      {modules.map((module) => {
        const active = currentGroup === module.group
        return (
          <button
            key={module.key}
            type="button"
            className={`rail-btn ${active ? 'active' : ''}`}
            title={module.label}
            onClick={() => module.first && navigate(module.first.path)}
          >
            {/* layoutId 让选中指示块在模块之间平滑移动。 */}
            {active && (
              <motion.span
                layoutId="rail-active"
                className="rail-active"
                aria-hidden="true"
                transition={reduceMotion ? { duration: 0 } : SPRING}
              />
            )}
            <span className="rail-glyph">
              <Icon name={module.icon} size={17} />
            </span>
          </button>
        )
      })}
      <div className="rail-spacer" />
      <button
        type="button"
        className="rail-btn rail-theme"
        title={theme === 'dark' ? '切换到浅色' : '切换到深色'}
        onClick={toggleTheme}
      >
        <span className="theme-swap">
          <span className={theme === 'dark' ? 'on' : ''}>
            <Icon name="sun" size={17} />
          </span>
          <span className={theme === 'light' ? 'on' : ''}>
            <Icon name="moon" size={17} />
          </span>
        </span>
      </button>
    </aside>
  )
}
