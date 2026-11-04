/**
 * 页签保活：已访问页面保持挂载，切换页签只隐藏，不丢失筛选条件与滚动位置。
 */
import { motion, useReducedMotion } from 'motion/react'
import { useRef, type ReactNode } from 'react'
import { useLocation, useOutlet } from 'react-router-dom'
import { useTabs } from '../../stores/tabs'
import { EASE_OUT } from '../../utils/motion'

interface KeepAliveOutletProps {
  refreshKey: number
}

export function KeepAliveOutlet({ refreshKey }: KeepAliveOutletProps) {
  const location = useLocation()
  const outlet = useOutlet()
  const tabs = useTabs((state) => state.tabs)
  const reduceMotion = useReducedMotion()
  const active = location.pathname

  // 缓存已渲染的页面元素；关闭页签时同步释放，避免内存持续增长。
  const pages = useRef(new Map<string, ReactNode>())
  // 每个页面独立的刷新版本号：只有当前页在“刷新”时重新挂载。
  const versions = useRef(new Map<string, number>())
  const lastRefresh = useRef(refreshKey)

  if (outlet) pages.current.set(active, outlet)
  if (lastRefresh.current !== refreshKey) {
    lastRefresh.current = refreshKey
    versions.current.set(active, (versions.current.get(active) ?? 0) + 1)
  }

  const keep = new Set(tabs.map((tab) => tab.key))
  keep.add(active)
  for (const key of pages.current.keys()) {
    if (!keep.has(key)) {
      pages.current.delete(key)
      versions.current.delete(key)
    }
  }

  return (
    <>
      {Array.from(pages.current.entries()).map(([path, node]) => {
        const version = versions.current.get(path) ?? 0
        return (
          <div key={`${path}:${version}`} style={{ display: path === active ? 'block' : 'none' }}>
            <motion.div
              className="page-stage"
              initial={path === active && !reduceMotion ? { opacity: 0.55, y: 7 } : false}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.22, ease: EASE_OUT }}
            >
              {node}
            </motion.div>
          </div>
        )
      })}
    </>
  )
}
