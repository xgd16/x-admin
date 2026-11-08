/**
 * 闲置自动锁屏：无输入达到设定时长后锁定，任意输入事件重置计时。
 */
import { useEffect } from 'react'
import { useLock } from '../stores/lock'
import { useUi } from '../stores/ui'

// capture 监听滚动等不冒泡事件；passive 避免阻塞滚动线程。
const RESET_EVENTS = ['pointermove', 'pointerdown', 'keydown', 'wheel', 'scroll'] as const

/** 同类事件的节流窗口：高频 pointermove 不必每次都重排计时器。 */
const RESET_THROTTLE_MS = 1_000

export function useIdleLock(): void {
  const minutes = useUi((state) => state.lockIdleMinutes)
  const locked = useLock((state) => state.locked)

  useEffect(() => {
    if (minutes <= 0 || locked) return
    let timer = 0
    let lastReset = 0
    const arm = () => {
      window.clearTimeout(timer)
      timer = window.setTimeout(() => useLock.getState().lock(), minutes * 60_000)
    }
    const reset = (event: Event) => {
      // isTrusted=false 的事件（脚本合成）不重置，避免程序化滚动把计时器永久续命。
      if (!event.isTrusted) return
      const now = Date.now()
      if (now - lastReset < RESET_THROTTLE_MS) return
      lastReset = now
      arm()
    }
    for (const type of RESET_EVENTS) {
      window.addEventListener(type, reset, { passive: true, capture: true })
    }
    arm()
    return () => {
      window.clearTimeout(timer)
      for (const type of RESET_EVENTS) {
        window.removeEventListener(type, reset, { capture: true })
      }
    }
  }, [minutes, locked])
}
