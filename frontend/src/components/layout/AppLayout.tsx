/**
 * 后台主框架：侧栏、命令栏、页签与页面容器。
 */
import { useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'
import { useSiteConfig } from '../../api/public'
import { useNav } from '../../config/nav'
import { useTabs } from '../../stores/tabs'
import { Ambient } from '../Ambient'
import { useGlassLight } from '../useGlassLight'
import { CommandBar } from './CommandBar'
import { KeepAliveOutlet } from './KeepAliveOutlet'
import { ContextBar } from './ContextBar'
import { Rail } from './Rail'
import { TabBar } from './TabBar'

export function AppLayout() {
  useGlassLight()
  useSiteConfig()
  const location = useLocation()
  const { byPath } = useNav()
  const open = useTabs((state) => state.open)
  const refreshKey = useTabs((state) => state.refreshKey)
  const canvasBodyRef = useRef<HTMLDivElement>(null)

  // 页签保活后按页面恢复滚动位置；首次进入的页面从顶部开始。
  const scrollPositions = useRef<Record<string, number>>({})
  useEffect(() => {
    const el = canvasBodyRef.current
    if (!el) return
    el.scrollTo({ top: scrollPositions.current[location.pathname] ?? 0 })
    return () => {
      scrollPositions.current[location.pathname] = el.scrollTop
    }
  }, [location.pathname])

  // 每次进入页面时确保对应页签已打开；登录页不参与页签体系。
  useEffect(() => {
    if (location.pathname === '/login') return
    const item = byPath(location.pathname)
    const profile = location.pathname === '/profile'
    open({
      key: location.pathname,
      label: profile ? '个人中心' : item.label,
      path: location.pathname,
    })
  }, [location.pathname, open, byPath])

  return (
    <div className="lg-app">
      <Ambient />
      <div className="lg-shell">
        <Rail />
        <ContextBar />
        <div className="canvas">
          <CommandBar />
          <TabBar />
          <div className="canvas-body" ref={canvasBodyRef}>
            {/* 已访问页面保持挂载；页签“刷新”只重新挂载当前页。 */}
            <KeepAliveOutlet refreshKey={refreshKey} />
          </div>
        </div>
      </div>
    </div>
  )
}
