import { useEffect, useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Checkbox, Input } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { Captcha } from '../api/types'
import { useAuth } from '../stores/auth'
import { tokenStore } from '../api/tokens'
import { Ambient } from '../components/Ambient'
import { Icon } from '../components/Icon'

export function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const login = useAuth((state) => state.login)
  const status = useAuth((state) => state.status)
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('Admin@123456')
  const [code, setCode] = useState('')
  const [remember, setRemember] = useState(true)
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)

  const captcha = useQuery({
    queryKey: ['captcha'],
    queryFn: () => api.get<Captcha>('/auth/captcha'),
    staleTime: 0,
    refetchOnWindowFocus: false,
  })

  useEffect(() => {
    if (status === 'ready' && tokenStore.access) {
      const from = (location.state as { from?: string } | null)?.from
      navigate(from && from !== '/login' ? from : '/dashboard', { replace: true })
    }
  }, [status, navigate, location.state])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!captcha.data) {
      setError('验证码尚未加载，请点击刷新')
      return
    }
    setPending(true)
    setError('')
    try {
      await login(username, password, captcha.data.captcha_id, code)
      const from = (location.state as { from?: string } | null)?.from
      navigate(from && from !== '/login' ? from : '/dashboard', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
      setCode('')
      void captcha.refetch()
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="lg-login">
      <Ambient />
      <div className="login-stage">
        <section className="login-hero glass">
          <div className="brand">
            <span className="logo">X</span>
            <span>
              <b>x-admin</b>
              <span>OPERATIONS WORKBENCH</span>
            </span>
          </div>
          <h1>
            回到工作台。
            <br />
            系统已经就绪。
          </h1>
          <p className="lede">用户、权限、任务与日志，在同一张桌面上。登录后从上次离开的地方继续。</p>
          <div className="login-orb" />
          <div className="login-spec">
            <span>
              API <b>42ms</b>
            </span>
            <span>
              MYSQL <b>8.4</b>
            </span>
            <span>
              REDIS <b>7.4</b>
            </span>
            <span>
              BUILD <b>v1.0.0</b>
            </span>
          </div>
        </section>

        <form className="login-card glass" onSubmit={submit}>
          <div className="form-kicker">ACCESS / 内部系统</div>
          <h2>系统登录</h2>
          <p className="form-sub">仅限授权账号 · 操作会写入审计日志</p>

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
              <button
                type="button"
                className="code"
                onClick={() => void captcha.refetch()}
                title="点击刷新验证码"
                style={{ padding: 0, background: 'transparent' }}
              >
                {captcha.data ? <img src={captcha.data.image} alt="验证码" /> : <span className="mono">{captcha.isFetching ? '···' : '刷新'}</span>}
              </button>
            </div>
          </div>

          <div className="form-meta">
            <Checkbox checked={remember} onChange={(event) => setRemember(event.target.checked)}>
              记住我
            </Checkbox>
            <a href="#reset" onClick={(event) => event.preventDefault()}>
              忘记密码
            </a>
          </div>

          {error && (
            <div style={{ margin: '-6px 0 12px', fontSize: 10.5, color: 'var(--danger)' }} role="alert">
              {error}
            </div>
          )}

          <button className="submit" type="submit" disabled={pending}>
            {pending ? '正在进入…' : '进入工作台'}
            <Icon name="arrowRight" size={14} />
          </button>
          <p className="hint">
            首次登录后请修改默认密码。<b>连续失败 5 次</b>将锁定 15 分钟。
          </p>
        </form>
      </div>
    </main>
  )
}
