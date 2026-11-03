import { Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import { motion, useReducedMotion } from 'motion/react'
import { useLocation, useNavigate } from 'react-router-dom'
import { navByPath, navItems, railGroups } from '../../config/nav'
import { useAuth } from '../../stores/auth'
import { useTabs } from '../../stores/tabs'
import { useUi } from '../../stores/ui'
import { SPRING } from '../../utils/motion'
import { Icon } from '../Icon'

export function Rail() {
  const navigate = useNavigate()
  const location = useLocation()
  const can = useAuth((state) => state.can)
  const user = useAuth((state) => state.user)
  const logout = useAuth((state) => state.logout)
  const resetTabs = useTabs((state) => state.reset)
  const theme = useUi((state) => state.theme)
  const toggleTheme = useUi((state) => state.toggleTheme)
  const reduceMotion = useReducedMotion()

  const currentGroup = navByPath(location.pathname).group
  const modules = railGroups
    .map((module) => ({
      ...module,
      first: navItems.find((item) => item.group === module.group && can(item.permission)),
    }))
    .filter((module) => module.first)

  const userMenu: MenuProps['items'] = [
    { key: 'profile', icon: <Icon name="user" size={14} />, label: '个人中心' },
    { type: 'divider' },
    { key: 'logout', icon: <Icon name="logout" size={14} />, label: '退出登录', danger: true },
  ]

  return (
    <aside className="rail glass-dark">
      <button className="rail-logo" type="button" onClick={() => navigate('/dashboard')} title="x-admin">
        X
      </button>
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
      <Dropdown
        trigger={['click']}
        placement="topRight"
        menu={{
          items: userMenu,
          onClick: ({ key }) => {
            if (key === 'profile') navigate('/profile')
            if (key === 'logout') {
              void logout().then(() => {
                resetTabs()
                navigate('/login')
              })
            }
          },
        }}
      >
        <button type="button" className="rail-avatar" title={user?.nickname ?? '账号'}>
          {(user?.nickname ?? user?.username ?? 'X').slice(0, 1)}
        </button>
      </Dropdown>
    </aside>
  )
}
