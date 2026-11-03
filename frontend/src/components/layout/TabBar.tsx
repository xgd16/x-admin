import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTabs } from '../../stores/tabs'
import { Icon } from '../Icon'

export function TabBar() {
  const navigate = useNavigate()
  const tabs = useTabs((state) => state.tabs)
  const active = useTabs((state) => state.active)
  const activate = useTabs((state) => state.activate)
  const close = useTabs((state) => state.close)
  const reset = useTabs((state) => state.reset)
  const activeRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [active, tabs.length])

  if (tabs.length <= 1) return null

  return (
    <div className="tab-strip">
      {tabs.map((tab) => (
        <div
          key={tab.key}
          ref={active === tab.key ? activeRef : undefined}
          className={`tab-chip ${active === tab.key ? 'active' : ''}`}
          onClick={() => {
            activate(tab.key)
            navigate(tab.path)
          }}
          role="button"
          tabIndex={0}
          onKeyDown={(event) => {
            if (event.key === 'Enter') navigate(tab.path)
          }}
        >
          <span>{tab.label}</span>
          <button
            type="button"
            className="close"
            aria-label={`关闭 ${tab.label}`}
            onClick={(event) => {
              event.stopPropagation()
              const next = close(tab.key)
              if (next) navigate(next.path)
              else if (tabs.length === 1) {
                reset()
                navigate('/dashboard')
              }
            }}
          >
            <Icon name="x" size={10} />
          </button>
        </div>
      ))}
    </div>
  )
}
