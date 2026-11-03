import { Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import { navItems } from '../../config/nav'
import { useAuth } from '../../stores/auth'
import { useTabs } from '../../stores/tabs'
import { useUi } from '../../stores/ui'
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

  const items = navItems.filter((item) => can(item.permission))
  const active = (path: string) => location.pathname === path || location.pathname.startsWith(`${path}/`)

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
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          className={`rail-btn ${active(item.path) ? 'active' : ''}`}
          title={item.label}
          onClick={() => navigate(item.path)}
        >
          <Icon name={item.icon} size={17} />
        </button>
      ))}
      <div className="rail-spacer" />
      <button type="button" className="rail-btn" title={theme === 'dark' ? '切换到浅色' : '切换到深色'} onClick={toggleTheme}>
        <Icon name={theme === 'dark' ? 'sun' : 'moon'} size={17} />
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
