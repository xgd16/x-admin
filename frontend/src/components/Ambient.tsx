import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

const LIVE_MS = 3000

/**
 * Decorative aurora layers are animated only for a short window after a route
 * change (and when the tab becomes visible again), then paused. Keeping the
 * huge blurred layers moving behind backdrop-filter panels costs a surprising
 * amount of GPU even when the UI is otherwise idle.
 */
export function Ambient() {
  const location = useLocation()

  useEffect(() => {
    const root = document.documentElement
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
    let timer: number | undefined

    const pause = () => {
      window.clearTimeout(timer)
      root.classList.remove('ambient-live')
    }

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
