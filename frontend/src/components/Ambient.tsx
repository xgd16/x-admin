/**
 * 背景极光装饰层，仅在路由切换后短暂播放动画。
 */
import { useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'

const LIVE_MS = 3000

/**
 * 背景极光装饰层。
 *
 * 这些大面积渐变图层只在路由切换后（以及标签页重新可见时）短暂播放动画，
 * 随后暂停；否则在 backdrop-filter 面板背后持续运动，会白白消耗大量 GPU。
 */
export function Ambient() {
  const location = useLocation()
  const playRef = useRef<() => void>(() => {})

  useEffect(() => {
    const root = document.documentElement
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
    let timer: number | undefined
    let live = false

    // 通过切换 html 上的 ambient-live 类启停 CSS 动画。
    const pause = () => {
      window.clearTimeout(timer)
      timer = undefined
      live = false
      root.classList.remove('ambient-live')
    }

    // 系统开启“减少动态效果”时直接跳过播放。
    // 已经在播放时不重新加类，避免频繁切页反复重启大图层动画造成掉帧。
    const play = () => {
      if (reduceMotion.matches || live) return
      live = true
      root.classList.add('ambient-live')
      timer = window.setTimeout(pause, LIVE_MS)
    }

    playRef.current = play

    const onVisibility = () => {
      if (document.visibilityState === 'visible') play()
      else pause()
    }

    play()
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      pause()
      playRef.current = () => {}
    }
  }, [])

  // 路由变化时尝试播放；若动画仍在播放窗口内则保持现状，不重置进度。
  useEffect(() => {
    playRef.current()
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
