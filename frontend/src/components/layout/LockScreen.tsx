/**
 * 全屏锁屏覆盖层：密码或通行密钥解锁，凭证由服务端重新校验。
 */
import { Input, type InputRef } from 'antd'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useLock } from '../../stores/lock'
import { useTabs } from '../../stores/tabs'
import { EASE_OUT } from '../../utils/motion'
import { isPasskeySupported } from '../../utils/passkey'
import { Ambient } from '../Ambient'
import { Icon } from '../Icon'
import { LoadingIndicator } from '../Loading'
import { useToast } from '../../hooks/useToast'

/** 锁定期间每秒刷新的时钟文案。 */
function useClock(locked: boolean) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    if (!locked) return
    setNow(new Date())
    const timer = window.setInterval(() => setNow(new Date()), 1000)
    return () => window.clearInterval(timer)
  }, [locked])
  return {
    time: now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }),
    date: now.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }),
  }
}

export function LockScreen() {
  const locked = useLock((state) => state.locked)
  const unlockWithPassword = useLock((state) => state.unlockWithPassword)
  const unlockWithPasskey = useLock((state) => state.unlockWithPasskey)
  const user = useAuth((state) => state.user)
  const logout = useAuth((state) => state.logout)
  const resetTabs = useTabs((state) => state.reset)
  const toast = useToast()
  const navigate = useNavigate()
  const reduceMotion = useReducedMotion()
  const inputRef = useRef<InputRef>(null)
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const [passkeyBusy, setPasskeyBusy] = useState(false)
  // 浏览器能力只探测一次，避免每次渲染触发权限提示检查。
  const [passkeyAvailable] = useState(() => isPasskeySupported())
  const { time, date } = useClock(locked)

  // 每次进入锁屏都清空表单并把焦点交给密码框；Esc 不允许关闭锁屏。
  useEffect(() => {
    if (!locked) return
    setPassword('')
    setError('')
    setPending(false)
    setPasskeyBusy(false)
    const frame = window.requestAnimationFrame(() => inputRef.current?.focus())
    return () => window.cancelAnimationFrame(frame)
  }, [locked])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (pending || passkeyBusy || !password) return
    setPending(true)
    setError('')
    try {
      await unlockWithPassword(password)
    } catch (err) {
      setError(err instanceof Error ? err.message : '密码验证失败，请重试')
    } finally {
      setPending(false)
    }
  }

  const unlockByPasskey = async () => {
    if (passkeyBusy || pending) return
    setPasskeyBusy(true)
    setError('')
    try {
      await unlockWithPasskey()
    } catch (err) {
      setError(err instanceof Error ? err.message : '通行密钥验证失败，请重试')
    } finally {
      setPasskeyBusy(false)
    }
  }

  // 逃生口：忘记密码且无通行密钥时可以放弃会话回到登录页。
  const signOut = () => {
    void logout().then(() => {
      resetTabs()
      navigate('/login')
      toast.success('已安全退出')
    })
  }

  return createPortal(
    <AnimatePresence>
      {locked && (
        <motion.div
          className="lock-screen"
          initial={reduceMotion ? false : { opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.26, ease: EASE_OUT }}
        >
          <Ambient />
          <motion.div
            className="lock-card glass"
            initial={reduceMotion ? false : { opacity: 0, y: 18, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            transition={{ duration: 0.32, ease: EASE_OUT, delay: 0.04 }}
          >
            <div className="lock-clock" aria-hidden="true">
              <span className="lock-time">{time}</span>
              <span className="lock-date">{date}</span>
            </div>
            <span className="avatar lock-avatar">{(user?.nickname ?? user?.username ?? 'X').slice(0, 1)}</span>
            <div className="lock-who">
              <b>{user?.nickname ?? user?.username ?? '管理员'}</b>
              <span>屏幕已锁定，请重新验证身份</span>
            </div>
            <form className="lock-form" onSubmit={submit}>
              <Input.Password
                size="large"
                ref={inputRef}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="输入密码解锁"
                autoComplete="current-password"
                disabled={pending || passkeyBusy}
              />
              <button className="lock-submit" type="submit" disabled={pending || !password} aria-busy={pending || undefined}>
                {pending && <LoadingIndicator size="sm" inverse />}
                {pending ? '正在验证…' : '解锁'}
                {!pending && <Icon name="arrowRight" size={14} />}
              </button>
            </form>
            {passkeyAvailable && (
              <button
                type="button"
                className="lock-passkey"
                onClick={() => void unlockByPasskey()}
                disabled={passkeyBusy || pending}
                aria-busy={passkeyBusy || undefined}
              >
                {passkeyBusy ? <LoadingIndicator size="sm" /> : <Icon name="key" size={14} />}
                {passkeyBusy ? '等待设备验证…' : '使用通行密钥解锁'}
              </button>
            )}
            <AnimatePresence>
              {error && (
                <motion.div
                  className="form-error"
                  role="alert"
                  initial={reduceMotion ? false : { opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.16, ease: EASE_OUT }}
                >
                  {error}
                </motion.div>
              )}
            </AnimatePresence>
            <button type="button" className="lock-logout" onClick={signOut}>
              退出登录
            </button>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  )
}
