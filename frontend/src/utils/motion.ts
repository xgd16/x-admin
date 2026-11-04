/**
 * 全局动效令牌，控制过渡时长与弹性参数。
 */
/**
 * 全局动效令牌。
 *
 * 页面大量使用大面积模糊玻璃层，持续动画会明显增加 GPU 开销，因此过渡应短促、
 * 一次性完成；这些值仅用于导航、页签和入场等状态变化。
 */
export const EASE_OUT = [0.22, 0.8, 0.28, 1] as const

export const SPRING = {
  type: 'spring',
  stiffness: 560,
  damping: 42,
  mass: 0.7,
} as const

export const DURATION = {
  fast: 0.16,
  base: 0.24,
  slow: 0.4,
} as const
