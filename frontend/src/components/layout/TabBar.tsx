/**
 * 页面多标签栏与右键菜单。
 */
import { useQueryClient } from '@tanstack/react-query'
import { Drawer } from 'antd'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useCallback, useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { useTabs, type Tab } from '../../stores/tabs'
import { useIsCompactViewport } from '../../hooks/useMediaQuery'
import { EASE_OUT, SPRING } from '../../utils/motion'
import { Icon } from '../Icon'

type MenuAction = 'refresh' | 'close' | 'others' | 'left' | 'right' | 'all'

interface TabMenuState {
  tab: Tab
  index: number
  x: number
  y: number
}

interface TabBarProps {
  /** 内容区下滑阅读时由 AppLayout 置真：页签栏收起让位，回滚时展开。 */
  autoHidden?: boolean
}

export function TabBar({ autoHidden = false }: TabBarProps) {
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
  const stripRef = useRef<HTMLDivElement>(null)
  const compact = useIsCompactViewport()
  const [scrollState, setScrollState] = useState({ left: false, right: false })
  const [tabSheetOpen, setTabSheetOpen] = useState(false)
  const [menu, setMenu] = useState<TabMenuState | null>(null)
  const visible = tabs.length > 1
  // 滚动收起只平移+淡出、保留占位高度：若收缩高度，吸顶组变矮会让内容高度
  // 变化，在“内容略高于视口”的区间 scrollTop 被钳回顶部，页签条随即展开，
  // 形成不停收起/展开的振荡环；单页签收起不走滚动判断，可安全收缩高度。
  const shown = visible && !autoHidden
  const activeTab = tabs.find((tab) => tab.key === active)

  // 记录页签条是否还能向左/向右滚动，用渐隐遮罩提示还有内容。
  const syncScrollState = useCallback(() => {
    const strip = stripRef.current
    if (!strip) return
    const left = strip.scrollLeft > 1
    const right = strip.scrollLeft + strip.clientWidth < strip.scrollWidth - 1
    setScrollState((current) => (current.left === left && current.right === right ? current : { left, right }))
  }, [])

  useEffect(() => {
    const strip = stripRef.current
    if (!strip) return
    syncScrollState()
    const observer = new ResizeObserver(syncScrollState)
    observer.observe(strip)
    window.addEventListener('resize', syncScrollState)
    return () => {
      observer.disconnect()
      window.removeEventListener('resize', syncScrollState)
    }
  }, [syncScrollState])

  // 页签数量/宽度变化后，等布局完成再更新一次左右滚动状态。
  useEffect(() => {
    const frame = window.requestAnimationFrame(syncScrollState)
    return () => window.cancelAnimationFrame(frame)
  }, [active, tabs.length, syncScrollState])

  // 旋转到桌面布局时关闭手机页签面板。
  useEffect(() => {
    if (!compact) setTabSheetOpen(false)
  }, [compact])

  // 只剩一个页签时无需页签切换面板。
  useEffect(() => {
    if (!visible) setTabSheetOpen(false)
  }, [visible])

  // 激活页签变化时保持它在可视区域内。
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [active, tabs.length])

  // 菜单打开时监听 Esc、外部点击、滚动和窗口尺寸变化，任一发生即关闭。
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
    // 估算菜单尺寸并夹在视口内，避免右键靠近边缘时溢出。
    const width = 184
    const height = 246
    setMenu({
      tab,
      index,
      x: Math.max(8, Math.min(event.clientX, window.innerWidth - width - 8)),
      y: Math.max(8, Math.min(event.clientY, window.innerHeight - height - 8)),
    })
  }

  // 关闭单个页签；关闭当前页时跳到右侧/左侧页签，全部关闭后回总览。
  const closeTab = (tab: Tab) => {
    const next = close(tab.key)
    if (next) navigate(next.path)
    else if (tabs.length === 1) {
      reset()
      navigate('/dashboard')
    }
  }

  // 统一处理右键菜单动作；关闭当前页后按 store 返回的下一 tab 跳转。
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
      closeTab(tab)
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

  // 手机页签面板：切换页签、关闭其他与全部关闭。
  const openTab = (tab: Tab) => {
    activate(tab.key)
    navigate(tab.path)
    setTabSheetOpen(false)
  }

  const closeOtherTabs = () => {
    if (!activeTab) return
    const next = closeOthers(activeTab.key)
    setTabSheetOpen(false)
    if (next) navigate(next.path)
  }

  const closeAllTabs = () => {
    const next = closeAll()
    setTabSheetOpen(false)
    navigate(next?.path ?? '/dashboard')
  }

  const enter = reduceMotion ? false : { opacity: 0, scale: 0.94 }
  const leave = reduceMotion ? { opacity: 1 } : { opacity: 0, scale: 0.9 }

  return (
    <>
      <motion.div
        className={`tab-strip-wrap ${scrollState.left ? 'has-left' : ''} ${scrollState.right ? 'has-right' : ''}`}
        initial={false}
        animate={{
          height: visible ? (compact ? 34 : 30) : 0,
          marginTop: visible ? 0 : -10,
          y: shown ? 0 : -8,
          opacity: shown ? 1 : 0,
        }}
        style={{ pointerEvents: shown ? 'auto' : 'none' }}
        transition={reduceMotion ? { duration: 0 } : { duration: 0.24, ease: EASE_OUT }}
      >
        {compact ? (
          visible ? (
            <div className="mobile-tabbar">
              {activeTab && (
                <div className="mobile-tab-chip">
                  <button
                    type="button"
                    className="mobile-tab-main"
                    onClick={() => setTabSheetOpen(true)}
                    aria-label={`当前页签 ${activeTab.label}，展开全部页签`}
                    aria-expanded={tabSheetOpen}
                  >
                    <span className="mobile-tab-label">{activeTab.label}</span>
                  </button>
                  <button
                    type="button"
                    className="mobile-tab-close"
                    aria-label={`关闭 ${activeTab.label}`}
                    onClick={() => closeTab(activeTab)}
                  >
                    <Icon name="x" size={10} />
                  </button>
                </div>
              )}
              <button
                type="button"
                className="mobile-tab-switch"
                onClick={() => setTabSheetOpen(true)}
                aria-label={`切换页签，共 ${tabs.length} 个`}
                aria-expanded={tabSheetOpen}
              >
                <Icon name="panel" size={14} />
                <span>{tabs.length}</span>
              </button>
            </div>
          ) : null
        ) : (
          <div className="tab-strip" ref={stripRef} onScroll={syncScrollState}>
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
                        closeTab(tab)
                      }}
                    >
                      <Icon name="x" size={10} />
                    </button>
                  </motion.div>
                ))}
            </AnimatePresence>
          </div>
        )}
      </motion.div>

      {compact && (
        <Drawer
          open={tabSheetOpen}
          onClose={() => setTabSheetOpen(false)}
          placement="bottom"
          size={`min(${180 + tabs.length * 54}px, 72dvh)`}
          closable={false}
          title={null}
          className="tab-sheet"
          styles={{ body: { padding: 0 } }}
          destroyOnHidden
        >
          <div className="tab-sheet-handle" aria-hidden="true" />
          <div className="tab-sheet-head">
            <span>打开的页签 · {tabs.length}</span>
            <button type="button" onClick={() => setTabSheetOpen(false)}>
              完成
            </button>
          </div>
          <div className="tab-sheet-list">
            {tabs.map((tab) => (
              <div className={`tab-sheet-item ${active === tab.key ? 'active' : ''}`} key={tab.key}>
                <button type="button" className="tab-sheet-main" onClick={() => openTab(tab)}>
                  <span className="tab-sheet-dot" aria-hidden="true" />
                  <span className="tab-sheet-label">{tab.label}</span>
                  <span className="tab-sheet-path">{tab.path}</span>
                </button>
                <button
                  type="button"
                  className="tab-sheet-close"
                  aria-label={`关闭 ${tab.label}`}
                  onClick={() => {
                    closeTab(tab)
                    if (tab.key === active) setTabSheetOpen(false)
                  }}
                >
                  <Icon name="x" size={12} />
                </button>
              </div>
            ))}
          </div>
          <div className="tab-sheet-foot">
            <button type="button" onClick={closeOtherTabs} disabled={tabs.length <= 1}>
              关闭其他
            </button>
            <button type="button" className="danger" onClick={closeAllTabs}>
              关闭全部
            </button>
          </div>
        </Drawer>
      )}

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
