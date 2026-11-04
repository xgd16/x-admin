/**
 * 背景极光装饰层，仅在路由切换后短暂播放动画。
 */
import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

const LIVE_MS = 3000

/**
 * 背景极光装饰层。
 *
 * 这些大面积模糊图层只在路由切换后（以及标签页重新可见时）短暂播放动画，
 * 随后暂停；否则在 backdrop-filter 面板背后持续运动，会白白消耗大量 GPU。
 */
export function Ambient() {
  const location = useLocation()

  useEffect(() => {
    const root = document.documentElement
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
    let timer: number | undefined

    // 通过切换 html 上的 ambient-live 类启停 CSS 动画。
    const pause = () => {
      window.clearTimeout(timer)
      root.classList.remove('ambient-live')
    }

    // 系统开启“减少动态效果”时直接跳过播放。
    const play = () => {
      if (reduceMotion.matches) return
      root.classList.add('ambient-live')
      window.clearTimeout(timer)
      timer = window.setTimeout(pause, LIVE_MS)
    }

    const onVisibility = () => {
      if (document.visibilityState === 'visible') play()
      else pause()
    }

    play()
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      pause()
    }
  }, [location.pathname])

  return (
    <div className="ambient" aria-hidden="true">
      <span className="blob blob-a" />
      <span className="blob blob-b" />
      <span className="blob blob-c" />
      <span className="blob blob-d" />
    </div>
  )
}
