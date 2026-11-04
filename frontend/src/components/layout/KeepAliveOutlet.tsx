/**
 * 页签保活：已访问页面保持挂载，切换页签只隐藏，不丢失筛选条件与滚动位置。
 */
import { useMemo, useRef, type ReactNode } from 'react'
import {
  UNSAFE_LocationContext,
  useLocation,
  useNavigationType,
  useOutlet,
} from 'react-router-dom'
import { useTabs } from '../../stores/tabs'

interface KeepAliveOutletProps {
  refreshKey: number
}

interface CachedPage {
  node: ReactNode
  // 保留页面激活时的路由快照，隐藏页不会读到当前页的地址与 query。
  context: { location: ReturnType<typeof useLocation>; navigationType: ReturnType<typeof useNavigationType> }
}

export function KeepAliveOutlet({ refreshKey }: KeepAliveOutletProps) {
  const location = useLocation()
  const navigationType = useNavigationType()
  const outlet = useOutlet()
  const tabs = useTabs((state) => state.tabs)
  const active = location.pathname

  // 缓存已渲染的页面元素；关闭页签时同步释放，避免内存持续增长。
  const pages = useRef(new Map<string, CachedPage>())
  // 每个页面独立的刷新版本号：只有当前页在“刷新”时重新挂载。
  const versions = useRef(new Map<string, number>())
  const lastRefresh = useRef(refreshKey)

  // 仅在地址变化时新建 context 对象，避免每次父组件渲染都触发所有缓存页更新。
  const context = useMemo(() => ({ location, navigationType }), [location, navigationType])

  if (outlet) pages.current.set(active, { node: outlet, context })
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
      {Array.from(pages.current.entries()).map(([path, page]) => {
        const version = versions.current.get(path) ?? 0
        const visible = path === active
        return (
          <div key={`${path}:${version}`} style={{ display: visible ? 'block' : 'none' }}>
            {/*
             * keep-alive 的隐藏页仍挂在同一棵 React 树上。给每个页面冻结激活时的
             * LocationContext，隐藏页就不会读到切换后的地址；未来若接入路由级
             * 缓存或页面级返回逻辑，也能保持各自的路由快照。
             */}
            <UNSAFE_LocationContext.Provider value={page.context}>
              <div className="page-stage">{page.node}</div>
            </UNSAFE_LocationContext.Provider>
          </div>
        )
      })}
    </>
  )
}
