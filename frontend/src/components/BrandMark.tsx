/**
 * x-admin 品牌标记（>_）SVG。
 */
import { useId } from 'react'

interface BrandMarkProps {
  size?: number
}

/**
 * x-admin 品牌标记：控制台提示符（`>_`），颜色跟随当前主题色板
 * （--accent-2 → --accent → --accent-3 渐变），在侧栏小尺寸下也能保持清晰。
 */
export function BrandMark({ size = 22 }: BrandMarkProps) {
  // SVG 渐变 id 必须全局唯一，避免同一页面多个 Logo 互相覆盖。
  const uid = useId().replace(/[^a-zA-Z0-9_-]/g, '')
  const chevron = `xa-chevron-${uid}`
  const cursor = `xa-cursor-${uid}`

  return (
    <svg viewBox="0 0 36 36" width={size} height={size} fill="none" aria-hidden="true">
      <defs>
        <linearGradient id={chevron} x1="7" y1="7" x2="20" y2="29" gradientUnits="userSpaceOnUse">
          {/* stopColor 走 CSS 变量；stop-color 表现属性不支持 var()，必须用内联样式。 */}
          <stop style={{ stopColor: 'var(--accent-2)' }} />
          <stop offset="0.5" style={{ stopColor: 'var(--accent)' }} />
          <stop offset="1" style={{ stopColor: 'var(--accent-3)' }} />
        </linearGradient>
        <linearGradient id={cursor} x1="21" y1="27" x2="30" y2="27" gradientUnits="userSpaceOnUse">
          <stop style={{ stopColor: 'var(--accent-2)' }} />
          <stop offset="1" style={{ stopColor: 'var(--accent-3)' }} />
        </linearGradient>
      </defs>
      <path
        d="M9 8.5 17.5 18 9 27.5"
        stroke={`url(#${chevron})`}
        strokeWidth="5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M21.5 27.5h8" stroke={`url(#${cursor})`} strokeWidth="5" strokeLinecap="round" />
    </svg>
  )
}
