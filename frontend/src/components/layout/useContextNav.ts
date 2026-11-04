/**
 * 计算当前模块的二级导航及展开偏好。
 */
import { useCallback } from 'react'
import { useLocation } from 'react-router-dom'
import { profileNav, useNav, type NavItem } from '../../config/nav'
import { useUi } from '../../stores/ui'

/**
 * 左侧模块对应的二级导航。
 *
 * 自动规则：只有 0 或 1 个页面的模块不需要二级面板，默认收起以让内容区占满宽度；
 * 多个页面时自动展开。用户手动收起/展开后，会为当前模块记录偏好；未操作过的模块
 * 继续遵循自动规则。
 */
export function useContextNav() {
  const location = useLocation()
  const { items: navItems, byPath } = useNav()
  const preference = useUi((state) => state.contextPref)
  const setContextPref = useUi((state) => state.setContextPref)

  const active = byPath(location.pathname)
  const isAccount = active.group === '账号'
  const items: NavItem[] = isAccount
    ? [profileNav]
    : navItems.filter((item) => item.group === active.group)
  // 有手动偏好时优先使用，否则按页面数量自动决定。
  const open = preference?.group === active.group ? preference.open : items.length > 1
  const toggle = useCallback(
    () => setContextPref(active.group, !open),
    [active.group, open, setContextPref],
  )

  return { active, items, open, toggle, isAccount }
}
