import { useEffect } from 'react'

/** Each glass surface catches the pointer light via CSS variables. */
export function useGlassLight() {
  useEffect(() => {
    let frame = 0
    let lastEvent: PointerEvent | null = null

    const apply = () => {
      frame = 0
      if (!lastEvent) return
      const target = (lastEvent.target as HTMLElement | null)?.closest<HTMLElement>('.glass')
      if (!target) return
      const rect = target.getBoundingClientRect()
      target.style.setProperty('--mx', `${(((lastEvent.clientX - rect.left) / rect.width) * 100).toFixed(1)}%`)
      target.style.setProperty('--my', `${(((lastEvent.clientY - rect.top) / rect.height) * 100).toFixed(1)}%`)
    }

    const onMove = (event: PointerEvent) => {
      lastEvent = event
      if (!frame) frame = requestAnimationFrame(apply)
    }
    document.addEventListener('pointermove', onMove)
    return () => {
      document.removeEventListener('pointermove', onMove)
      if (frame) cancelAnimationFrame(frame)
    }
  }, [])
}
