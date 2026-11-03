import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTabs } from '../../stores/tabs'
import { EASE_OUT, SPRING } from '../../utils/motion'
import { Icon } from '../Icon'

export function TabBar() {
  const navigate = useNavigate()
  const tabs = useTabs((state) => state.tabs)
  const active = useTabs((state) => state.active)
  const activate = useTabs((state) => state.activate)
  const close = useTabs((state) => state.close)
  const reset = useTabs((state) => state.reset)
  const reduceMotion = useReducedMotion()
  const activeRef = useRef<HTMLDivElement>(null)
  const visible = tabs.length > 1

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [active, tabs.length])

  const enter = reduceMotion ? false : { opacity: 0, scale: 0.94 }
  const leave = reduceMotion ? { opacity: 1 } : { opacity: 0, scale: 0.9 }

  return (
    <motion.div
      className="tab-strip-wrap"
      initial={false}
      animate={{
        height: visible ? 30 : 0,
        marginTop: visible ? 0 : -10,
        opacity: visible ? 1 : 0,
      }}
      style={{ pointerEvents: visible ? 'auto' : 'none' }}
      transition={reduceMotion ? { duration: 0 } : { duration: 0.24, ease: EASE_OUT }}
    >
      <div className="tab-strip">
        <AnimatePresence initial={false} mode="popLayout">
          {visible &&
            tabs.map((tab) => (
              <motion.div
                layout="position"
                key={tab.key}
                ref={active === tab.key ? activeRef : undefined}
                className={`tab-chip ${active === tab.key ? 'active' : ''}`}
                initial={enter}
                animate={{ opacity: 1, scale: 1 }}
                exit={leave}
                transition={
                  reduceMotion
                    ? { duration: 0 }
                    : { duration: 0.18, ease: EASE_OUT, layout: SPRING }
                }
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
              </motion.div>
            ))}
        </AnimatePresence>
      </div>
    </motion.div>
  )
}
