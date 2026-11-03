import { motion, useReducedMotion } from 'motion/react'
import { useEffect } from 'react'
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
  const reduceMotion = useReducedMotion()

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
          <div className="canvas-body">
            <motion.div
              key={location.pathname}
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
