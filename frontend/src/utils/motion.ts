/**
 * Shared motion tokens.
 *
 * Keep transitions short and one-shot: this UI relies on large blurred glass
 * layers, so anything that runs forever after navigation is expensive. These
 * values are only used for state changes (navigation, tabs, entrance).
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
