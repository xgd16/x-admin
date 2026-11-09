/**
 * 后台主框架：侧栏、命令栏、页签与页面容器。
 */
import { useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'
import { useSiteConfig } from '../../api/public'
import { useNav } from '../../config/nav'
import { useCanvasScrollSurface } from '../../hooks/useCanvasScrollSurface'
import { useScrollAutoHide } from '../../hooks/useScrollAutoHide'
import { useIdleLock } from '../../hooks/useIdleLock'
import { useTabs } from '../../stores/tabs'
import { bindLockAcrossTabs } from '../../stores/lock'
import { Ambient } from '../Ambient'
import { CommandBar } from './CommandBar'
import { KeepAliveOutlet } from './KeepAliveOutlet'
import { ContextBar } from './ContextBar'
import { LockScreen } from './LockScreen'
import { Rail } from './Rail'
import { TabBar } from './TabBar'

export function AppLayout() {
  useSiteConfig()
  useIdleLock()
  const location = useLocation()
  const { byPath } = useNav()
  const open = useTabs((state) => state.open)
  const refreshKey = useTabs((state) => state.refreshKey)
  const canvasBodyRef = useRef<HTMLDivElement>(null)

  // 其他标签页锁定/解锁时联动本标签页。
  useEffect(() => bindLockAcrossTabs(), [])
  // 下滑阅读内容时收起页签栏，回滚或回到顶部时展开。
  const [tabsHidden, markProgrammaticScroll] = useScrollAutoHide(canvasBodyRef)
  // 内容滚到命令栏下方时才开启实时磨砂，停在顶部时省掉无视觉收益的背景采样。
  useCanvasScrollSurface(canvasBodyRef)

  // 页签保活后按页面恢复滚动位置；首次进入的页面从顶部开始。
  const scrollPositions = useRef<Record<string, number>>({})
  useEffect(() => {
    const el = canvasBodyRef.current
    if (!el) return
    // 恢复阅读位置是程序化滚动，页签栏保持展开，不计入收起判断。
    markProgrammaticScroll()
    el.scrollTo({ top: scrollPositions.current[location.pathname] ?? 0 })
    return () => {
      scrollPositions.current[location.pathname] = el.scrollTop
    }
  }, [location.pathname, markProgrammaticScroll])

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
          <div className="canvas-body" ref={canvasBodyRef}>
            {/* 命令栏与页签栏组成吸顶悬浮组，内容滚动时从玻璃卡片后方穿过；
                页签栏在下滑阅读时自动收起，回滚再滑出。 */}
            <div className="canvas-head">
              <CommandBar />
              <TabBar autoHidden={tabsHidden} />
            </div>
            <div className="canvas-pad">
              {/* 已访问页面保持挂载；页签“刷新”只重新挂载当前页。 */}
              <KeepAliveOutlet refreshKey={refreshKey} />
            </div>
          </div>
        </div>
      </div>
      {/* 锁屏覆盖层挂在 body 上，盖住侧栏、命令栏与所有页面。 */}
      <LockScreen />
    </div>
  )
}
