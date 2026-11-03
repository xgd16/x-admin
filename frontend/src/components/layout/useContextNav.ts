import { useCallback } from 'react'
import { useLocation } from 'react-router-dom'
import { itemsOfGroup, navByPath, profileNav, type NavItem } from '../../config/nav'
import { useAuth } from '../../stores/auth'
import { useUi } from '../../stores/ui'

/**
 * Second-level navigation for the module selected in the rail.
 *
 * Auto behaviour: a module with zero or one page does not need a second-level
 * panel, so it stays collapsed and the canvas gets the full width. A module
 * with several pages opens automatically. The collapse button stores a manual
 * preference per module; switching modules resets it to the automatic rule.
 */
export function useContextNav() {
  const location = useLocation()
  const can = useAuth((state) => state.can)
  const preference = useUi((state) => state.contextPref)
  const setContextPref = useUi((state) => state.setContextPref)

  const active = navByPath(location.pathname)
  const isAccount = active.group === '账号'
  const items: NavItem[] = isAccount
    ? [profileNav]
    : itemsOfGroup(active.group).filter((item) => can(item.permission))
  const open = preference?.group === active.group ? preference.open : items.length > 1
  const toggle = useCallback(
    () => setContextPref(active.group, !open),
    [active.group, open, setContextPref],
  )

  return { active, items, open, toggle, isAccount }
}
