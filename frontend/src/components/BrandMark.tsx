import { useId } from 'react'

interface BrandMarkProps {
  size?: number
}

/**
 * The x-admin mark: a console prompt (`>_`) drawn with a warm gradient. It
 * reads as a workbench/command surface rather than a close icon, and stays
 * legible at rail size.
 */
export function BrandMark({ size = 22 }: BrandMarkProps) {
  const uid = useId().replace(/[^a-zA-Z0-9_-]/g, '')
  const chevron = `xa-chevron-${uid}`
  const cursor = `xa-cursor-${uid}`

  return (
    <svg viewBox="0 0 36 36" width={size} height={size} fill="none" aria-hidden="true">
      <defs>
        <linearGradient id={chevron} x1="7" y1="7" x2="20" y2="29" gradientUnits="userSpaceOnUse">
          <stop stopColor="#ffb38f" />
          <stop offset="0.5" stopColor="#ff6a3d" />
          <stop offset="1" stopColor="#ff4d8d" />
        </linearGradient>
        <linearGradient id={cursor} x1="21" y1="27" x2="30" y2="27" gradientUnits="userSpaceOnUse">
          <stop stopColor="#ff9a6c" />
          <stop offset="1" stopColor="#ff3d6e" />
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
