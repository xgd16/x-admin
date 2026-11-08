/**
 * 滚动方向监听：内容区向下阅读时收起悬浮条，回滚或接近顶部时展开。
 */
import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'

/** 距顶部小于该值时常显，避免刚进入页面就收起。 */
const NEAR_TOP = 40
/** 累计向下滚动超过该距离才收起，过滤滚轮抖动与惯性微滚。 */
const HIDE_AFTER = 20

/**
 * 监听滚动容器的垂直滚动方向，返回 [是否应收起, 标记程序化滚动]。
 * 程序化滚动（路由切换恢复阅读位置）不属于阅读行为，页签栏保持展开。
 */
export function useScrollAutoHide(target: RefObject<HTMLElement | null>): [boolean, () => void] {
  const [hidden, setHidden] = useState(false)
  const lastYRef = useRef(0)
  const accumRef = useRef(0)
  const programmaticUntilRef = useRef(0)

  const markProgrammaticScroll = useCallback(() => {
    // 复位/恢复最多在当帧产生一次 scroll 事件；用时间窗而非“武装待消费”标记，
    // 目标位置与当前相同、事件不触发时窗口自动过期，不会吞掉用户的真实滚动。
    programmaticUntilRef.current = performance.now() + 120
  }, [])

  useEffect(() => {
    const el = target.current
    if (!el) return
    const onScroll = () => {
      const y = el.scrollTop
      const dy = y - lastYRef.current
      lastYRef.current = y
      if (performance.now() < programmaticUntilRef.current) {
        accumRef.current = 0
        setHidden(false)
        return
      }
      if (y <= NEAR_TOP) {
        accumRef.current = 0
        setHidden(false)
        return
      }
      // 小步滚动逐次累加，只有持续向下超过阈值才收起；任一方向触发后重新累计。
      const accum = Math.max(-HIDE_AFTER, Math.min(HIDE_AFTER, accumRef.current + dy))
      if (accum >= HIDE_AFTER) {
        accumRef.current = 0
        setHidden(true)
      } else if (accum <= -HIDE_AFTER) {
        accumRef.current = 0
        setHidden(false)
      } else {
        accumRef.current = accum
      }
    }
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => el.removeEventListener('scroll', onScroll)
  }, [target])

  return [hidden, markProgrammaticScroll]
}
