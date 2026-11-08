/**
 * 顶部命令栏：面包屑、全局搜索、通知、设置与用户菜单。
 */
import { useEffect, useMemo, useState } from 'react'
import { Drawer, Dropdown, Input, Modal, Popover, Segmented, Switch } from 'antd'
import type { MenuProps } from 'antd'
import { useNavigate } from 'react-router-dom'
import { useNav } from '../../config/nav'
import { useAuth } from '../../stores/auth'
import { useLock } from '../../stores/lock'
import { useTabs } from '../../stores/tabs'
import { useUi } from '../../stores/ui'
import { Icon } from '../Icon'
import { useToast } from '../../hooks/useToast'
import { NotificationCenter } from './NotificationCenter'
import { ThemePalettePicker } from './ThemePalettePicker'
import { useContextNav } from './useContextNav'

export function CommandBar() {
  const navigate = useNavigate()
  const { active, items: contextItems, open: contextOpen, toggle: toggleContext } = useContextNav()
  const user = useAuth((state) => state.user)
  const { items: navItems } = useNav()
  const logout = useAuth((state) => state.logout)
  const resetTabs = useTabs((state) => state.reset)
  const theme = useUi((state) => state.theme)
  const toggleTheme = useUi((state) => state.toggleTheme)
  const density = useUi((state) => state.density)
  const setDensity = useUi((state) => state.setDensity)
  const watermark = useUi((state) => state.watermark)
  const toggleWatermark = useUi((state) => state.toggleWatermark)
  const lockIdleMinutes = useUi((state) => state.lockIdleMinutes)
  const setLockIdleMinutes = useUi((state) => state.setLockIdleMinutes)
  const lockScreen = useLock((state) => state.lock)
  const toast = useToast()

  const [paletteOpen, setPaletteOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [mobileNav, setMobileNav] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

  // 全局快捷键：⌘/Ctrl+K 打开快速导航，Esc 关闭。
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setPaletteOpen((open) => !open)
      }
      if (event.key === 'Escape') setPaletteOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // 快速导航按名称、路径和分组做在前端过滤，数据量小，无需请求后端。
  const results = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return navItems.filter(
      (item) => !keyword || `${item.label}${item.path}${item.group}`.toLowerCase().includes(keyword),
    )
  }, [query, navItems])

  const userMenu: MenuProps['items'] = [
    { key: 'profile', icon: <Icon name="user" size={14} />, label: '个人中心' },
    { key: 'settings', icon: <Icon name="settings" size={14} />, label: '偏好设置' },
    { key: 'lock', icon: <Icon name="lock" size={14} />, label: '锁定屏幕' },
    { type: 'divider' },
    { key: 'logout', icon: <Icon name="logout" size={14} />, label: '退出登录', danger: true },
  ]

  // 偏好设置弹层：主题、主题色、表格密度、水印与快捷键提示。
  const settings = (
    <div className="settings-menu">
      <div className="settings-row">
        <span className="label">外观</span>
        <Segmented
          size="small"
          value={theme}
          options={[
            { label: '浅色', value: 'light' },
            { label: '深色', value: 'dark' },
          ]}
          onChange={(value) => {
            if (value !== theme) toggleTheme()
          }}
        />
      </div>
      <ThemePalettePicker />
      <div className="settings-row">
        <span className="label">表格密度</span>
        <Segmented
          size="small"
          value={density}
          options={[
            { label: '紧', value: 'small' },
            { label: '中', value: 'middle' },
            { label: '松', value: 'large' },
          ]}
          onChange={(value) => setDensity(value as 'small' | 'middle' | 'large')}
        />
      </div>
      <div className="settings-row">
        <span className="label">全局水印</span>
        <Switch size="small" checked={watermark} onChange={toggleWatermark} />
      </div>
      <div className="settings-row">
        <span className="label">自动锁屏</span>
        <Segmented
          size="small"
          value={lockIdleMinutes}
          options={[
            { label: '关闭', value: 0 },
            { label: '5分', value: 5 },
            { label: '10分', value: 10 },
            { label: '30分', value: 30 },
          ]}
          onChange={(value) => setLockIdleMinutes(value as number)}
        />
      </div>
      <div className="settings-row">
        <span className="label">快捷键</span>
        <span className="value">⌘K 导航</span>
      </div>
    </div>
  )

  return (
    <>
      <header className="command glass">
        <button type="button" className="icon-btn mobile-nav-btn" onClick={() => setMobileNav(true)} aria-label="导航">
          <Icon name="menu" size={16} />
        </button>
        {contextItems.length > 0 && (
          <button
            type="button"
            className={`icon-btn ctx-toggle ${contextOpen ? 'on' : ''}`}
            onClick={toggleContext}
            title={contextOpen ? '收起二级导航' : '展开二级导航'}
            aria-label={contextOpen ? '收起二级导航' : '展开二级导航'}
            aria-expanded={contextOpen}
          >
            <Icon name="panel" size={16} />
          </button>
        )}
        <div className="crumb">
          {active.group} / <b>{active.label}</b>
        </div>
        <button type="button" className="cmd" onClick={() => setPaletteOpen(true)}>
          <span>搜索用户、菜单、日志…</span>
          <kbd>⌘K</kbd>
        </button>
        <div className="cmd-actions">
          <button type="button" className="icon-btn" title="锁定屏幕" aria-label="锁定屏幕" onClick={lockScreen}>
            <Icon name="lock" size={16} />
          </button>
          <NotificationCenter />
          <Popover
            trigger="click"
            open={settingsOpen}
            onOpenChange={setSettingsOpen}
            placement="bottomRight"
            content={settings}
          >
            <button type="button" className="icon-btn" title="偏好设置">
              <Icon name="settings" size={16} />
            </button>
          </Popover>
        </div>
        <Dropdown
          trigger={['click']}
          placement="bottomRight"
          menu={{
            items: userMenu,
            onClick: ({ key }) => {
              if (key === 'profile') navigate('/profile')
              if (key === 'settings') setSettingsOpen(true)
              if (key === 'lock') lockScreen()
              if (key === 'logout') {
                void logout().then(() => {
                  resetTabs()
                  navigate('/login')
                  // 先落到位再提示，toast 直接以登录页的位置出现，不跳动。
                  toast.success('已安全退出')
                })
              }
            },
          }}
        >
          <div className="cmd-user">
            <span className="avatar">{(user?.nickname ?? user?.username ?? 'X').slice(0, 1)}</span>
            <span className="who">
              <b>{user?.nickname ?? user?.username ?? '管理员'}</b>
              {user?.roles?.some((role) => role.code === 'super_admin') ? '超级管理员' : '已登录'}
            </span>
            <Icon name="chevronDown" size={12} />
          </div>
        </Dropdown>
      </header>

      {/* 快速导航面板：输入关键字后直接跳转页面。 */}
      <Modal
        open={paletteOpen}
        onCancel={() => setPaletteOpen(false)}
        footer={null}
        width={560}
        title={null}
        closable={false}
        styles={{ body: { paddingTop: 8 } }}
      >
        <Input
          autoFocus
          variant="borderless"
          autoComplete="off"
          prefix={<Icon name="search" size={16} />}
          suffix={
            <button
              type="button"
              className="palette-esc"
              onClick={() => setPaletteOpen(false)}
              aria-label="关闭快速导航"
              title="关闭快速导航（Esc）"
            >
              ESC
            </button>
          }
          placeholder="搜索页面、模块…"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          style={{ fontSize: 14, padding: '8px 4px' }}
        />
        <div className="palette" style={{ marginTop: 8 }}>
          <div className="kicker">Navigate</div>
          {results.map((item) => (
            <button
              key={item.key}
              type="button"
              className={active.key === item.key ? 'active' : ''}
              onClick={() => {
                navigate(item.path)
                setPaletteOpen(false)
                setQuery('')
              }}
            >
              <Icon name={item.icon} size={15} />
              <span>{item.label}</span>
              <span className="meta">{item.group}</span>
            </button>
          ))}
          {results.length === 0 && <div className="empty-hint">没有匹配的页面</div>}
        </div>
      </Modal>

      {/* 小屏导航抽屉：复用桌面端的导航配置。 */}
      <Drawer
        open={mobileNav}
        onClose={() => setMobileNav(false)}
        placement="left"
        size={250}
        title={<span style={{ fontFamily: 'var(--mono)', fontSize: 10 }}>X-ADMIN / WORKSPACE</span>}
      >
        <div className="palette">
          {navItems.map((item) => (
            <button
              key={item.key}
              type="button"
              className={active.key === item.key ? 'active' : ''}
              onClick={() => {
                navigate(item.path)
                setMobileNav(false)
              }}
            >
              <Icon name={item.icon} size={15} />
              <span>{item.label}</span>
              <span className="meta">{item.group}</span>
            </button>
          ))}
          <button type="button" onClick={() => { navigate('/profile'); setMobileNav(false) }}>
            <Icon name="user" size={15} />
            <span>个人中心</span>
          </button>
        </div>
      </Drawer>
    </>
  )
}
