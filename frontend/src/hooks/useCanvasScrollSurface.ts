/**
 * 内容区滚动状态：给根节点写入 data-canvas-scrolled / data-canvas-scrolling。
 *
 * - canvas-scrolled：内容已经滚到命令栏下方，静态磨砂玻璃保持开启。
 * - canvas-scrolling：用户正在滚动。实时 backdrop-filter 在滚动期间必须
 *   逐帧重新采样整块背景，在 Apple Silicon 的 tile GPU 上开销尤其明显；
 *   因此滚动时改用高密度静态霜面，停下后立即恢复真正的背景模糊。
 */
import { useEffect, type RefObject } from 'react'

/** 最后一次滚动事件后多久视为“已停下”。 */
const SCROLL_IDLE_MS = 140

export function useCanvasScrollSurface(target: RefObject<HTMLElement | null>): void {
  useEffect(() => {
    const element = target.current
    if (!element) return
    const root = document.documentElement
    let idleTimer: number | undefined

    const setAttr = (name: 'canvasScrolled' | 'canvasScrolling', value: 'on' | 'off') => {
      if (root.dataset[name] !== value) root.dataset[name] = value
    }

    const sync = () => {
      // 1px 容差：部分缩放比例下 scrollTop 会停留在 0.5 附近。
      setAttr('canvasScrolled', element.scrollTop > 1 ? 'on' : 'off')
    }

    const onScroll = () => {
      sync()
      setAttr('canvasScrolling', 'on')
      window.clearTimeout(idleTimer)
      idleTimer = window.setTimeout(() => setAttr('canvasScrolling', 'off'), SCROLL_IDLE_MS)
    }

    sync()
    element.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      element.removeEventListener('scroll', onScroll)
      window.clearTimeout(idleTimer)
      delete root.dataset.canvasScrolled
      delete root.dataset.canvasScrolling
    }
  }, [target])
}
