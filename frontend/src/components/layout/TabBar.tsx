import { useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { useTabs, type Tab } from '../../stores/tabs'
import { EASE_OUT, SPRING } from '../../utils/motion'
import { Icon } from '../Icon'

type MenuAction = 'refresh' | 'close' | 'others' | 'left' | 'right' | 'all'

interface TabMenuState {
  tab: Tab
  index: number
  x: number
  y: number
}

export function TabBar() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const tabs = useTabs((state) => state.tabs)
  const active = useTabs((state) => state.active)
  const activate = useTabs((state) => state.activate)
  const close = useTabs((state) => state.close)
  const closeOthers = useTabs((state) => state.closeOthers)
  const closeLeft = useTabs((state) => state.closeLeft)
  const closeRight = useTabs((state) => state.closeRight)
  const closeAll = useTabs((state) => state.closeAll)
  const refresh = useTabs((state) => state.refresh)
  const reset = useTabs((state) => state.reset)
  const reduceMotion = useReducedMotion()
  const activeRef = useRef<HTMLDivElement>(null)
  const [menu, setMenu] = useState<TabMenuState | null>(null)
  const visible = tabs.length > 1

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [active, tabs.length])

  useEffect(() => {
    if (!menu) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenu(null)
    }
    const onPointer = (event: PointerEvent) => {
      if (!(event.target as HTMLElement | null)?.closest('.tab-menu')) setMenu(null)
    }
    const closeMenu = () => setMenu(null)
    window.addEventListener('keydown', onKey)
    window.addEventListener('pointerdown', onPointer)
    window.addEventListener('resize', closeMenu)
    window.addEventListener('scroll', closeMenu, true)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('pointerdown', onPointer)
      window.removeEventListener('resize', closeMenu)
      window.removeEventListener('scroll', closeMenu, true)
    }
  }, [menu])

  useEffect(() => {
    setMenu(null)
  }, [active, tabs.length])

  const openMenu = (event: ReactMouseEvent, tab: Tab, index: number) => {
    event.preventDefault()
    event.stopPropagation()
    const width = 184
    const height = 246
    setMenu({
      tab,
      index,
      x: Math.max(8, Math.min(event.clientX, window.innerWidth - width - 8)),
      y: Math.max(8, Math.min(event.clientY, window.innerHeight - height - 8)),
    })
  }

  const runAction = (action: MenuAction) => {
    if (!menu) return
    const tab = menu.tab
    setMenu(null)
    if (action === 'refresh') {
      activate(tab.key)
      navigate(tab.path)
      refresh()
      void queryClient.invalidateQueries()
      return
    }
    if (action === 'close') {
      const next = close(tab.key)
      if (next) navigate(next.path)
      else if (tabs.length === 1) {
        reset()
        navigate('/dashboard')
      }
      return
    }
    if (action === 'others') {
      const next = closeOthers(tab.key)
      if (next) navigate(next.path)
      return
    }
    if (action === 'left') {
      const next = closeLeft(tab.key)
      if (next) navigate(next.path)
      return
    }
    if (action === 'right') {
      const next = closeRight(tab.key)
      if (next) navigate(next.path)
      return
    }
    const next = closeAll()
    navigate(next?.path ?? '/dashboard')
  }

  const enter = reduceMotion ? false : { opacity: 0, scale: 0.94 }
  const leave = reduceMotion ? { opacity: 1 } : { opacity: 0, scale: 0.9 }

  return (
    <>
      <motion.div
        className="tab-strip-wrap"
        initial={false}
        animate={{
          height: visible ? 30 : 0,
          marginTop: visible ? 0 : -10,
          opacity: visible ? 1 : 0,
        }}
        style={{ pointerEvents: visible ? 'auto' : 'none' }}
        transition={reduceMotion ? { duration: 0 } : { duration: 0.24, ease: EASE_OUT }}
      >
        <div className="tab-strip">
          <AnimatePresence initial={false} mode="popLayout">
            {visible &&
              tabs.map((tab, index) => (
                <motion.div
                  layout="position"
                  key={tab.key}
                  ref={active === tab.key ? activeRef : undefined}
                  className={`tab-chip ${active === tab.key ? 'active' : ''}`}
                  initial={enter}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={leave}
                  transition={
                    reduceMotion
                      ? { duration: 0 }
                      : { duration: 0.18, ease: EASE_OUT, layout: SPRING }
                  }
                  onClick={() => {
                    activate(tab.key)
                    navigate(tab.path)
                  }}
                  onContextMenu={(event) => openMenu(event, tab, index)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') navigate(tab.path)
                  }}
                >
                  <span>{tab.label}</span>
                  <button
                    type="button"
                    className="close"
                    aria-label={`关闭 ${tab.label}`}
                    onClick={(event) => {
                      event.stopPropagation()
                      const next = close(tab.key)
                      if (next) navigate(next.path)
                      else if (tabs.length === 1) {
                        reset()
                        navigate('/dashboard')
                      }
                    }}
                  >
                    <Icon name="x" size={10} />
                  </button>
                </motion.div>
              ))}
          </AnimatePresence>
        </div>
      </motion.div>

      {menu &&
        createPortal(
          <div
            className="tab-menu-layer"
            onPointerDown={() => setMenu(null)}
            onContextMenu={(event) => event.preventDefault()}
          >
            <motion.div
              className="tab-menu"
              style={{ left: menu.x, top: menu.y }}
              initial={reduceMotion ? false : { opacity: 0, scale: 0.96, y: -4 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              transition={{ duration: 0.16, ease: EASE_OUT }}
              role="menu"
              aria-label={`${menu.tab.label} 标签页操作`}
              onPointerDown={(event) => event.stopPropagation()}
            >
              <button type="button" role="menuitem" autoFocus onClick={() => runAction('refresh')}>
                <Icon name="refresh" size={13} />
                <span>刷新</span>
              </button>
              <button type="button" role="menuitem" onClick={() => runAction('close')}>
                <Icon name="x" size={13} />
                <span>关闭</span>
              </button>
              <div className="tab-menu-sep" role="separator" />
              <button type="button" role="menuitem" onClick={() => runAction('others')}>
                <Icon name="close" size={13} />
                <span>关闭其他</span>
              </button>
              <button
                type="button"
                role="menuitem"
                disabled={menu.index === 0}
                onClick={() => runAction('left')}
              >
                <Icon name="chevronLeft" size={13} />
                <span>关闭左侧</span>
              </button>
              <button
                type="button"
                role="menuitem"
                disabled={menu.index === tabs.length - 1}
                onClick={() => runAction('right')}
              >
                <Icon name="chevronRight" size={13} />
                <span>关闭右侧</span>
              </button>
              <div className="tab-menu-sep" role="separator" />
              <button type="button" role="menuitem" className="danger" onClick={() => runAction('all')}>
                <Icon name="trash" size={13} />
                <span>关闭全部</span>
              </button>
            </motion.div>
          </div>,
          document.body,
        )}
    </>
  )
}
