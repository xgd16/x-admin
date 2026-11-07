/**
 * 媒体查询 Hook：让组件在渲染层跟随视口/指针类型切换布局。
 */
import { useEffect, useState } from 'react'

/** 手机、窄平板与触屏窄窗口统一视为紧凑布局。 */
export const COMPACT_MEDIA_QUERY = '(max-width: 900px), (pointer: coarse) and (max-width: 1024px)'

/** 当前视口是否匹配任意 CSS 媒体查询。 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window === 'undefined') return false
    return window.matchMedia(query).matches
  })

  useEffect(() => {
    const media = window.matchMedia(query)
    const update = () => setMatches(media.matches)

    // 查询字符串变化时重新同步，避免首帧使用旧值。
    update()
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [query])

  return matches
}

/** 是否使用紧凑布局（手机、平板或触屏窄窗口）。 */
export function useIsCompactViewport(): boolean {
  return useMediaQuery(COMPACT_MEDIA_QUERY)
}
