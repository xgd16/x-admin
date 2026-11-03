import { motion, useReducedMotion } from 'motion/react'
import { useEffect, useRef } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { navByPath } from '../../config/nav'
import { useTabs } from '../../stores/tabs'
import { EASE_OUT } from '../../utils/motion'
import { Ambient } from '../Ambient'
import { useGlassLight } from '../useGlassLight'
import { CommandBar } from './CommandBar'
import { ContextBar } from './ContextBar'
import { Rail } from './Rail'
import { TabBar } from './TabBar'

export function AppLayout() {
  useGlassLight()
  const location = useLocation()
  const open = useTabs((state) => state.open)
  const refreshKey = useTabs((state) => state.refreshKey)
  const reduceMotion = useReducedMotion()
  const canvasBodyRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    canvasBodyRef.current?.scrollTo({ top: 0 })
  }, [location.pathname])

  useEffect(() => {
    if (location.pathname === '/login') return
    const item = navByPath(location.pathname)
    const profile = location.pathname === '/profile'
    open({
      key: location.pathname,
      label: profile ? '个人中心' : item.label,
      path: location.pathname,
    })
  }, [location.pathname, open])

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
            <motion.div
              key={`${location.pathname}::${refreshKey}`}
              className="page-stage"
              initial={reduceMotion ? false : { opacity: 0.55, y: 7 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.22, ease: EASE_OUT }}
            >
              <Outlet />
            </motion.div>
          </div>
        </div>
      </div>
    </div>
  )
}
