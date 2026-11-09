/**
 * 内容区滚动状态：给根节点写入 data-canvas-scrolled。
 *
 * 命令栏只在内容滚到它下方时启用实时磨砂。停在页面顶部时背后是静态极光，
 * 模糊几乎不可见，却会让合成器每帧重新采样背景；滚起来之后再保持玻璃感。
 */
import { useEffect, type RefObject } from 'react'

export function useCanvasScrollSurface(target: RefObject<HTMLElement | null>): void {
  useEffect(() => {
    const element = target.current
    if (!element) return
    const root = document.documentElement

    const sync = () => {
      // 1px 容差：部分缩放比例下 scrollTop 会停留在 0.5 附近。
      const next = element.scrollTop > 1 ? 'on' : 'off'
      if (root.dataset.canvasScrolled !== next) root.dataset.canvasScrolled = next
    }

    sync()
    element.addEventListener('scroll', sync, { passive: true })
    return () => {
      element.removeEventListener('scroll', sync)
      delete root.dataset.canvasScrolled
    }
  }, [target])
}
