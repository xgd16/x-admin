/**
 * 登录页：账号密码、验证码、MFA 与通行密钥登录。
 */
import { useEffect, useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { App, Checkbox, Input } from 'antd'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import type { Captcha } from '../api/types'
import { useAuth } from '../stores/auth'
import { tokenStore } from '../api/tokens'
import { Ambient } from '../components/Ambient'
import { BrandMark } from '../components/BrandMark'
import { Icon } from '../components/Icon'
import { EASE_OUT } from '../utils/motion'
import { isPasskeySupported, passkeyErrorMessage } from '../utils/passkey'

/** 根据当前时间返回登录页的问候文案。 */
function loginCopy() {
  const hour = new Date().getHours()
  if (hour < 5) {
    return {
      title: '夜深了，',
      line: '早点休息吧。',
      lede: '如果还有没做完的事，明天再继续也不迟。',
    }
  }
  if (hour < 12) {
    return {
      title: '早上好，',
      line: '今天也从容一点。',
      lede: '先坐下来，喝口水。把今天要处理的事，一件一件做好。',
    }
  }
  if (hour < 18) {
    return {
      title: '下午好，',
      line: '还有时间，慢慢来。',
      lede: '工作不必一口气做完，按自己的节奏就好。',
    }
  }
  return {
    title: '晚上好，',
    line: '今天也辛苦了。',
    lede: '收个尾，把桌面整理好。剩下的事，明天再继续。',
  }
}

export function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const { message } = App.useApp()
  const login = useAuth((state) => state.login)
  const verifyMFA = useAuth((state) => state.verifyMFA)
  const passkeyLogin = useAuth((state) => state.passkeyLogin)
  const status = useAuth((state) => state.status)
  const copy = loginCopy()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [remember, setRemember] = useState(true)
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const [passkeyBusy, setPasskeyBusy] = useState(false)
  const [mfaToken, setMfaToken] = useState('')
  const [otp, setOtp] = useState('')
  const [recoveryMode, setRecoveryMode] = useState(false)
  const passkeyAvailable = isPasskeySupported()
  const reduceMotion = useReducedMotion()

  // 图形验证码进入页面即加载；点击图片会重新获取。
  const captcha = useQuery({
    queryKey: ['captcha'],
    queryFn: () => api.get<Captcha>('/auth/captcha'),
    staleTime: 0,
    refetchOnWindowFocus: false,
  })

  // 已登录用户访问登录页时，直接回到来源页或总览。
  useEffect(() => {
    if (status === 'ready' && tokenStore.access) {
      const from = (location.state as { from?: string } | null)?.from
      navigate(from && from !== '/login' ? from : '/dashboard', { replace: true })
    }
  }, [status, navigate, location.state])

  // 第一步：提交账号密码与验证码；若后端要求 MFA，则切到动态验证码表单。
  const submitCredentials = async (event: FormEvent) => {
    event.preventDefault()
    if (!captcha.data) {
      setError('验证码尚未加载，请点击刷新')
      return
    }
    setPending(true)
    setError('')
    try {
      const result = await login(username, password, captcha.data.captcha_id, code)
      if (result.mfa_required && result.mfa_token) {
        setMfaToken(result.mfa_token)
        setOtp('')
        return
      }
      goNext()
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
      setCode('')
      void captcha.refetch()
    } finally {
      setPending(false)
    }
  }

  // 第二步：提交 6 位动态验证码或恢复码；mfa_token 过期时退回账号密码登录。
  const submitMFA = async (event: FormEvent) => {
    event.preventDefault()
    if (!otp.trim()) {
      setError(recoveryMode ? '请输入恢复码' : '请输入 6 位动态验证码')
      return
    }
    setPending(true)
    setError('')
    try {
      await verifyMFA(mfaToken, otp.trim())
      goNext()
    } catch (err) {
      setError(err instanceof Error ? err.message : '验证失败')
      setOtp('')
      if (err instanceof ApiError && err.code === 40102) {
        setMfaToken('')
        setRecoveryMode(false)
        void captcha.refetch()
      }
    } finally {
      setPending(false)
    }
  }

  /** 登录成功后回到拦截前的页面，默认进入总览。 */
  const goNext = () => {
    const from = (location.state as { from?: string } | null)?.from
    navigate(from && from !== '/login' ? from : '/dashboard', { replace: true })
  }

  const backToCredentials = () => {
    setMfaToken('')
    setOtp('')
    setRecoveryMode(false)
    setError('')
    void captcha.refetch()
  }

  // 通行密钥登录由浏览器唤起指纹 / 面容 / 设备 PIN。
  const handlePasskeyLogin = async () => {
    setPending(true)
    setPasskeyBusy(true)
    setError('')
    try {
      await passkeyLogin()
      goNext()
    } catch (err) {
      setError(passkeyErrorMessage(err))
    } finally {
      setPending(false)
      setPasskeyBusy(false)
    }
  }

  return (
    <main className="lg-login">
      <Ambient />
      <div className="login-stage">
        <motion.section
          className="login-hero glass"
          initial={reduceMotion ? false : { opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, ease: EASE_OUT }}
        >
          <div className="brand">
            <span className="logo">
              <BrandMark size={23} />
            </span>
            <span>
              <b>x-admin</b>
              <span>EVERYDAY WORKSPACE</span>
            </span>
          </div>
          <h1>
            {copy.title}
            <br />
            {copy.line}
          </h1>
          <p className="lede">{copy.lede}</p>
          <div className="login-orb" />
        </motion.section>

        <motion.form
          className="login-card glass"
          onSubmit={mfaToken ? submitMFA : submitCredentials}
          initial={reduceMotion ? false : { opacity: 0, y: 18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: reduceMotion ? 0 : 0.08, ease: EASE_OUT }}
        >
          {/* mfaToken 存在时显示第二因子表单，否则显示账号密码表单。 */}
          {mfaToken ? (
            <>
              <div className="form-kicker">TWO-FACTOR / 两步验证</div>
              <h2>动态验证码</h2>
              <p className="form-sub">
                打开 Google Authenticator，输入当前{recoveryMode ? '恢复码' : ' 6 位验证码'}
              </p>

              <div className="field">
                <label htmlFor="otp">{recoveryMode ? '恢复码' : '动态验证码'}</label>
                <Input
                  id="otp"
                  size="large"
                  className="otp-input"
                  autoFocus
                  value={otp}
                  inputMode={recoveryMode ? 'text' : 'numeric'}
                  autoComplete="one-time-code"
                  maxLength={recoveryMode ? 9 : 6}
                  onChange={(event) => {
                    const raw = event.target.value
                    setOtp(recoveryMode ? raw.toUpperCase() : raw.replace(/\D/g, ''))
                  }}
                  placeholder={recoveryMode ? 'XXXX-XXXX' : '000000'}
                />
              </div>

              <div className="login-switch">
                <button
                  type="button"
                  className="link-btn"
                  onClick={() => {
                    setRecoveryMode((value) => !value)
                    setOtp('')
                    setError('')
                  }}
                >
                  {recoveryMode ? '使用动态验证码' : '使用恢复码'}
                </button>
                <button type="button" className="link-btn" onClick={backToCredentials}>
                  返回重新登录
                </button>
              </div>
            </>
          ) : (
            <>
              <div className="form-kicker">WELCOME BACK / 欢迎回来</div>
              <h2>登录</h2>
              <p className="form-sub">请使用你的办公账号登录</p>

              <div className="field">
                <label htmlFor="username">用户名</label>
                <Input
                  id="username"
                  size="large"
                  value={username}
                  autoComplete="username"
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder="请输入用户名"
                />
              </div>
              <div className="field">
                <label htmlFor="password">密码</label>
                <Input.Password
                  id="password"
                  size="large"
                  value={password}
                  autoComplete="current-password"
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder="请输入密码"
                />
              </div>
              <div className="field">
                <label htmlFor="captcha">验证码</label>
                <div className="captcha">
                  <Input
                    id="captcha"
                    size="large"
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    placeholder="输入验证码"
                    maxLength={6}
                  />
                  {/* 点击验证码图片刷新；加载中显示省略号。 */}
                  <button
                    type="button"
                    className="code"
                    onClick={() => void captcha.refetch()}
                    title="点击刷新验证码"
                    style={{ padding: 0, background: 'transparent' }}
                  >
                    {captcha.data ? (
                      <motion.img
                        key={captcha.data.captcha_id}
                        src={captcha.data.image}
                        alt="验证码"
                        initial={reduceMotion ? false : { opacity: 0, scale: 0.96 }}
                        animate={{ opacity: 1, scale: 1 }}
                        transition={{ duration: 0.2, ease: EASE_OUT }}
                      />
                    ) : (
                      <span className="mono">{captcha.isFetching ? '···' : '刷新'}</span>
                    )}
                  </button>
                </div>
              </div>

              <div className="form-meta">
                <Checkbox checked={remember} onChange={(event) => setRemember(event.target.checked)}>
                  记住我
                </Checkbox>
                <a
                  href="#reset"
                  onClick={(event) => {
                    event.preventDefault()
                    message.info('请联系管理员重置密码')
                  }}
                >
                  忘记密码
                </a>
              </div>
            </>
          )}

          <AnimatePresence initial={false}>
            {error && (
              <motion.div
                className="form-error"
                role="alert"
                initial={reduceMotion ? false : { opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={reduceMotion ? { opacity: 0 } : { opacity: 0, y: -4 }}
                transition={{ duration: 0.18, ease: EASE_OUT }}
              >
                {error}
              </motion.div>
            )}
          </AnimatePresence>

          <button className="submit" type="submit" disabled={pending}>
            {pending ? (mfaToken ? '正在验证…' : '正在进入…') : mfaToken ? '验证并进入' : '进入工作台'}
            <Icon name="arrowRight" size={14} />
          </button>
          {!mfaToken && passkeyAvailable && (
            <>
              <div className="login-divider">
                <span>或</span>
              </div>
              <button
                type="button"
                className="passkey-login"
                disabled={pending}
                onClick={() => void handlePasskeyLogin()}
              >
                <Icon name="key" size={14} />
                {passkeyBusy ? '等待设备验证…' : '使用通行密钥登录'}
              </button>
            </>
          )}
          <p className="hint">
            {mfaToken ? (
              <>
                动态验证码每 30 秒更新一次。<b>连续输错 5 次</b>需重新登录。
              </>
            ) : (
              <>
                登录遇到问题请联系管理员。<b>连续失败 5 次</b>将锁定 15 分钟。
              </>
            )}
          </p>
        </motion.form>
      </div>
    </main>
  )
}
