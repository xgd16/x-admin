/**
 * 个人中心：资料、密码、TOTP 与通行密钥管理。
 */
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input, Modal } from 'antd'
import { api } from '../api/client'
import { tokenStore } from '../api/tokens'
import type { PasskeyItem, PasskeyRegistrationOptions, TOTPSetup, TOTPStatus, User } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { FileSelect } from '../components/FileSelect'
import { Icon } from '../components/Icon'
import { formatDate, initials } from '../utils/format'
import { createPasskey, isPasskeySupported, passkeyErrorMessage } from '../utils/passkey'

interface ProfileValues {
  nickname: string
  email: string
  phone: string
  avatar: string
}

interface TOTPFormValues {
  password: string
  otp_code: string
}

export function Profile() {
  const { message, modal } = App.useApp()
  const queryClient = useQueryClient()
  const storeUser = useAuth((state) => state.user)
  const can = useAuth((state) => state.can)
  const canUseFileLibrary = can('system:file:list')
  const [profileForm] = Form.useForm<ProfileValues>()
  const [passwordForm] = Form.useForm<{ old_password: string; new_password: string; confirm: string }>()
  const [enableForm] = Form.useForm<TOTPFormValues>()
  const [disableForm] = Form.useForm<TOTPFormValues>()
  const [enableOpen, setEnableOpen] = useState(false)
  const [disableOpen, setDisableOpen] = useState(false)
  const [setup, setSetup] = useState<TOTPSetup | null>(null)
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [passkeyForm] = Form.useForm<{ name: string; password: string }>()
  const [renameForm] = Form.useForm<{ name: string }>()
  const [passkeyOpen, setPasskeyOpen] = useState(false)
  const [renameTarget, setRenameTarget] = useState<PasskeyItem | null>(null)

  // 个人资料优先使用接口最新数据，接口未返回时回退到登录态里的用户。
  const query = useQuery({
    queryKey: ['profile'],
    queryFn: () => api.get<User>('/profile'),
    staleTime: 30_000,
  })

  const user = query.data ?? storeUser

  // TOTP 与通行密钥状态独立查询，任一变化后分别刷新。
  const totpQuery = useQuery({
    queryKey: ['profile-totp'],
    queryFn: () => api.get<TOTPStatus>('/profile/totp'),
    staleTime: 30_000,
  })

  const passkeysQuery = useQuery({
    queryKey: ['profile-passkeys'],
    queryFn: () => api.get<PasskeyItem[]>('/profile/passkeys'),
    staleTime: 30_000,
  })

  // —— 基本资料与密码 ——
  const updateMutation = useMutation({
    mutationFn: (values: ProfileValues) => api.put<User>('/profile', values),
    onSuccess: (updated) => {
      message.success('个人资料已更新')
      profileForm.setFieldsValue({ ...updated, avatar: updated.avatar ?? '' })
      void queryClient.invalidateQueries({ queryKey: ['profile'] })
      void useAuth.getState().bootstrap()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const passwordMutation = useMutation({
    mutationFn: (values: { old_password: string; new_password: string }) => api.put('/profile/password', values),
    onSuccess: () => {
      message.success('密码已修改，下次登录请使用新密码')
      passwordForm.resetFields()
    },
    onError: (error: Error) => message.error(error.message),
  })

  // TOTP 开关会影响登录态里的 totp_enabled，因此同时刷新 auth store。
  const refreshTOTP = () => {
    void queryClient.invalidateQueries({ queryKey: ['profile-totp'] })
    void queryClient.invalidateQueries({ queryKey: ['profile'] })
    void useAuth.getState().bootstrap()
  }

  // —— TOTP：获取密钥/二维码 -> 输入验证码启用 -> 保存恢复码 ——
  const setupMutation = useMutation({
    mutationFn: () => api.post<TOTPSetup>('/profile/totp/setup'),
    onSuccess: (data) => {
      setSetup(data)
      setRecoveryCodes([])
      enableForm.resetFields()
      setEnableOpen(true)
    },
    onError: (error: Error) => message.error(error.message),
  })

  const enableMutation = useMutation({
    mutationFn: (values: TOTPFormValues) =>
      api.post<{ recovery_codes: string[] }>('/profile/totp/enable', {
        ...values,
        refresh_token: tokenStore.refresh,
      }),
    onSuccess: (data) => {
      setRecoveryCodes(data.recovery_codes)
      message.success('双重验证已开启')
      refreshTOTP()
    },
    onError: (error: Error) => message.error(error.message),
  })

  const disableMutation = useMutation({
    mutationFn: (values: TOTPFormValues) => api.del('/profile/totp', values),
    onSuccess: () => {
      message.success('双重验证已关闭')
      setDisableOpen(false)
      disableForm.resetFields()
      refreshTOTP()
    },
    onError: (error: Error) => message.error(error.message),
  })

  // —— 通行密钥：begin 取挑战 -> 浏览器创建凭证 -> finish 落库 ——
  const addPasskeyMutation = useMutation({
    mutationFn: async (values: { name: string; password: string }) => {
      const begin = await api.post<PasskeyRegistrationOptions>('/profile/passkeys/begin', {
        password: values.password,
      })
      const credential = await createPasskey(begin)
      return api.post<PasskeyItem>('/profile/passkeys/finish', {
        session_id: begin.session_id,
        name: values.name,
        credential,
      })
    },
    onSuccess: (created) => {
      message.success('通行密钥已添加')
      setPasskeyOpen(false)
      passkeyForm.resetFields()
      queryClient.setQueryData<PasskeyItem[]>(['profile-passkeys'], (current) => [
        created,
        ...(current ?? []).filter((item) => item.id !== created.id),
      ])
      void queryClient.invalidateQueries({ queryKey: ['profile-passkeys'] })
    },
    onError: (error: unknown) => message.error(passkeyErrorMessage(error)),
  })

  const renamePasskeyMutation = useMutation({
    mutationFn: (values: { id: number; name: string }) =>
      api.put(`/profile/passkeys/${values.id}`, { name: values.name }),
    onSuccess: (_data, variables) => {
      message.success('设备名称已更新')
      setRenameTarget(null)
      renameForm.resetFields()
      queryClient.setQueryData<PasskeyItem[]>(['profile-passkeys'], (current) =>
        (current ?? []).map((item) => (item.id === variables.id ? { ...item, name: variables.name } : item)),
      )
      void queryClient.invalidateQueries({ queryKey: ['profile-passkeys'] })
    },
    onError: (error: Error) => message.error(error.message),
  })

  const deletePasskeyMutation = useMutation({
    mutationFn: (id: number) => api.del(`/profile/passkeys/${id}`),
    onSuccess: (_data, id) => {
      message.success('通行密钥已删除')
      queryClient.setQueryData<PasskeyItem[]>(['profile-passkeys'], (current) =>
        (current ?? []).filter((item) => item.id !== id),
      )
      void queryClient.invalidateQueries({ queryKey: ['profile-passkeys'] })
    },
    onError: (error: Error) => message.error(error.message),
  })

  const enabled = totpQuery.data?.enabled ?? user?.totp_enabled ?? false
  const recoveryLeft = totpQuery.data?.recovery_codes_left ?? 0
  const passkeys = passkeysQuery.data ?? []
  const passkeysSupported = isPasskeySupported()

  const openPasskey = () => {
    passkeyForm.resetFields()
    passkeyForm.setFieldsValue({ name: '我的设备' })
    setPasskeyOpen(true)
  }

  const closePasskey = () => {
    if (addPasskeyMutation.isPending) return
    setPasskeyOpen(false)
    passkeyForm.resetFields()
  }

  const confirmDeletePasskey = (item: PasskeyItem) => {
    modal.confirm({
      title: `删除「${item.name}」？`,
      content: '删除后，这台设备将无法再使用通行密钥登录。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => deletePasskeyMutation.mutateAsync(item.id),
    })
  }

  // 优先使用 Clipboard API；非安全上下文回退到临时 textarea。
  const copyText = async (text: string, label: string) => {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text)
      } else {
        const area = document.createElement('textarea')
        area.value = text
        area.style.position = 'fixed'
        area.style.opacity = '0'
        document.body.appendChild(area)
        area.select()
        document.execCommand('copy')
        area.remove()
      }
      message.success(label)
    } catch {
      message.error('复制失败，请手动选择文本')
    }
  }

  const closeEnable = () => {
    if (enableMutation.isPending) return
    setEnableOpen(false)
    setSetup(null)
    setRecoveryCodes([])
    enableForm.resetFields()
  }

  return (
    <>
      <PageHeader
        title="个人中心"
        sub="账号资料 · 安全设置 · 最近登录信息"
        actions={
          <span className="stat-chip">
            <Icon name="shield" size={11} /> 当前会话已加密
          </span>
        }
      />

      {/* 左侧身份卡，右侧依次为资料、双重验证、通行密钥与密码。 */}
      <div className="profile-grid">
        <aside className="profile-card glass">
          <div className="profile-avatar">
            {user?.avatar ? (
              <img src={user.avatar} alt={user.nickname || user.username} />
            ) : (
              initials(user?.nickname || user?.username)
            )}
          </div>
          <b style={{ display: 'block', fontSize: 16 }}>{user?.nickname || user?.username || '—'}</b>
          <span className="mono">{user?.email || '未设置邮箱'}</span>
          <div className="profile-meta">
            <div className="row">
              <span>账号</span>
              <b>{user?.username}</b>
            </div>
            <div className="row">
              <span>部门</span>
              <b>{user?.dept?.name || '—'}</b>
            </div>
            <div className="row">
              <span>角色</span>
              <b>{user?.roles?.map((role) => role.name).join(' / ') || '—'}</b>
            </div>
            <div className="row">
              <span>最后登录</span>
              <b>{formatDate(user?.last_login_at)}</b>
            </div>
            <div className="row">
              <span>登录 IP</span>
              <b>{user?.last_login_ip || '—'}</b>
            </div>
          </div>
        </aside>

        <div style={{ display: 'grid', gap: 10 }}>
          <section className="panel glass">
            <div className="panel-head">
              <h2>基本资料</h2>
              <span>PROFILE</span>
            </div>
            <Form
              form={profileForm}
              layout="vertical"
              style={{ marginTop: 12 }}
              initialValues={{
                nickname: user?.nickname,
                email: user?.email,
                phone: user?.phone,
                avatar: user?.avatar,
              }}
              onFinish={(values) => updateMutation.mutate(values)}
            >
              <div className="detail-grid">
                <Form.Item name="nickname" label="昵称">
                  <Input placeholder="显示名称" />
                </Form.Item>
                <Form.Item name="email" label="邮箱">
                  <Input placeholder="name@example.com" />
                </Form.Item>
                <Form.Item name="phone" label="手机号">
                  <Input placeholder="13800000000" />
                </Form.Item>
                <Form.Item name="avatar" label="头像">
                  {canUseFileLibrary ? (
                    <FileSelect types="image" placeholder="选择头像" title="选择头像" />
                  ) : (
                    <Input placeholder="https://…" />
                  )}
                </Form.Item>
              </div>
              <Button type="primary" htmlType="submit" loading={updateMutation.isPending}>
                保存资料
              </Button>
            </Form>
          </section>

          <section className="panel glass">
            <div className="panel-head">
              <h2>双重验证</h2>
              <span>GOOGLE AUTH</span>
            </div>
            <div className="totp-panel">
              <div className="totp-copy">
                <span className={`totp-badge ${enabled ? 'on' : ''}`}>
                  <Icon name={enabled ? 'shield' : 'key'} size={12} />
                  {enabled ? '已开启' : '未开启'}
                </span>
                <p>
                  使用 Google Authenticator 生成动态验证码，登录时在密码之后再验证一次。
                  即使密码泄露，账号也不会被直接登录。
                </p>
                {enabled ? (
                  <span className="totp-meta">
                    恢复码剩余 {recoveryLeft} / 10
                    {recoveryLeft <= 3 ? '，建议关闭后重新绑定以生成新的恢复码' : ''}
                  </span>
                ) : (
                  <span className="totp-meta">开启后请妥善保存恢复码，手机丢失时仍可登录。</span>
                )}
              </div>
              {enabled ? (
                <Button
                  danger
                  onClick={() => {
                    disableForm.resetFields()
                    setDisableOpen(true)
                  }}
                >
                  关闭双重验证
                </Button>
              ) : (
                <Button type="primary" loading={setupMutation.isPending} onClick={() => setupMutation.mutate()}>
                  启用 Google 验证
                </Button>
              )}
            </div>
          </section>

          <section className="panel glass">
            <div className="panel-head">
              <h2>通行密钥</h2>
              <span>PASSKEY</span>
            </div>
            <div className="passkey-panel">
              <div className="passkey-copy">
                <span className={`totp-badge ${passkeys.length > 0 ? 'on' : ''}`}>
                  <Icon name="key" size={12} />
                  {passkeys.length > 0 ? `${passkeys.length} 台设备` : '未配置'}
                </span>
                <p>
                  使用指纹、面容或设备 PIN 直接登录，不再输入密码。可以在手机、平板、电脑等多台设备上分别添加。
                </p>
                {passkeysSupported ? (
                  <span className="totp-meta">
                    通行密钥保存在设备上，服务器只保存公钥。最多绑定 10 台，可与双重验证同时使用。
                  </span>
                ) : (
                  <span className="totp-meta">
                    当前浏览器或访问地址不支持通行密钥，请使用 localhost 或 HTTPS 域名访问。
                  </span>
                )}
              </div>
              <Button type="primary" disabled={!passkeysSupported} onClick={openPasskey}>
                添加通行密钥
              </Button>
            </div>
            <div className="passkey-list">
              {passkeysQuery.isLoading ? (
                <div className="passkey-empty">正在读取设备…</div>
              ) : passkeys.length === 0 ? (
                <div className="passkey-empty">还没有通行密钥。添加后可以在登录页免密码进入。</div>
              ) : (
                passkeys.map((item) => (
                  <div className="passkey-item" key={item.id}>
                    <span className="passkey-icon">
                      <Icon name="key" size={15} />
                    </span>
                    <div className="passkey-info">
                      <b>{item.name}</b>
                      <span>
                        添加于 {formatDate(item.created_at)} ·{' '}
                        {item.last_used_at ? `最近使用 ${formatDate(item.last_used_at)}` : '还未使用过'}
                      </span>
                    </div>
                    <div className="passkey-actions">
                      <button
                        type="button"
                        onClick={() => {
                          setRenameTarget(item)
                          renameForm.setFieldsValue({ name: item.name })
                        }}
                      >
                        重命名
                      </button>
                      <button type="button" className="danger" onClick={() => confirmDeletePasskey(item)}>
                        删除
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>

          <section className="panel glass">
            <div className="panel-head">
              <h2>修改密码</h2>
              <span>SECURITY</span>
            </div>
            <Form
              form={passwordForm}
              layout="vertical"
              style={{ marginTop: 12 }}
              onFinish={(values) => {
                if (values.new_password !== values.confirm) {
                  message.error('两次输入的新密码不一致')
                  return
                }
                passwordMutation.mutate({ old_password: values.old_password, new_password: values.new_password })
              }}
            >
              <div className="detail-grid">
                <Form.Item name="old_password" label="原密码" rules={[{ required: true, message: '请输入原密码' }]}>
                  <Input.Password placeholder="当前密码" />
                </Form.Item>
                <Form.Item name="new_password" label="新密码" rules={[{ required: true, message: '请输入新密码' }]}>
                  <Input.Password placeholder="至少 8 位，含字母和数字" />
                </Form.Item>
                <Form.Item name="confirm" label="确认新密码" rules={[{ required: true, message: '请再次输入新密码' }]}>
                  <Input.Password placeholder="再次输入" />
                </Form.Item>
              </div>
              <Button type="primary" htmlType="submit" loading={passwordMutation.isPending}>
                更新密码
              </Button>
            </Form>
          </section>
        </div>
      </div>

      {/* TOTP 绑定弹窗：启用成功后切换为恢复码展示。 */}
      <Modal open={enableOpen} footer={null} width={600} onCancel={closeEnable} title={null}>
        {recoveryCodes.length > 0 ? (
          <div className="totp-recovery">
            <div className="form-kicker">RECOVERY CODES / 恢复码</div>
            <h3>请立即保存这 10 个恢复码</h3>
            <p className="totp-hint">
              每个恢复码只能使用一次。手机丢失或验证器被删除时，可用其中一个完成登录。关闭窗口后将无法再次查看。
            </p>
            <div className="totp-code-grid">
              {recoveryCodes.map((recovery) => (
                <code key={recovery}>{recovery}</code>
              ))}
            </div>
            <div className="totp-modal-actions">
              <Button onClick={() => void copyText(recoveryCodes.join('\n'), '恢复码已复制')}>复制全部</Button>
              <Button type="primary" onClick={closeEnable}>
                我已保存
              </Button>
            </div>
          </div>
        ) : setup ? (
          <div className="totp-setup">
            <div className="form-kicker">SET UP / 绑定 Google Authenticator</div>
            <h3>扫码绑定</h3>
            <div className="totp-qr">
              <img src={setup.qr_code} alt="Google Authenticator 绑定二维码" />
              <div className="totp-qr-side">
                <ol className="totp-steps">
                  <li>打开 Google Authenticator，点击右下角的 +。</li>
                  <li>选择「扫描二维码」，扫描左侧图片。</li>
                  <li>输入手机上的 6 位验证码，完成绑定。</li>
                </ol>
                <div className="totp-secret">
                  <span>无法扫码？手动添加密钥</span>
                  <code>{setup.secret}</code>
                  <button type="button" onClick={() => void copyText(setup.secret, '密钥已复制')}>
                    复制
                  </button>
                </div>
              </div>
            </div>
            <Form form={enableForm} layout="vertical" onFinish={(values) => enableMutation.mutate(values)}>
              <div className="detail-grid">
                <Form.Item name="password" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
                  <Input.Password placeholder="确认是你本人操作" autoComplete="current-password" />
                </Form.Item>
                <Form.Item
                  name="otp_code"
                  label="6 位验证码"
                  normalize={(value: string) => value.replace(/\D/g, '')}
                  rules={[
                    { required: true, message: '请输入验证码' },
                    { len: 6, message: '验证码为 6 位数字' },
                  ]}
                >
                  <Input placeholder="000000" maxLength={6} inputMode="numeric" autoComplete="one-time-code" />
                </Form.Item>
              </div>
              <div className="totp-modal-actions">
                <Button onClick={closeEnable}>取消</Button>
                <Button type="primary" htmlType="submit" loading={enableMutation.isPending}>
                  完成绑定
                </Button>
              </div>
            </Form>
          </div>
        ) : null}
      </Modal>

      {/* 关闭 TOTP 需要密码 + 动态验证码/恢复码双重确认。 */}
      <Modal
        open={disableOpen}
        footer={null}
        width={480}
        title="关闭双重验证"
        onCancel={() => {
          if (disableMutation.isPending) return
          setDisableOpen(false)
          disableForm.resetFields()
        }}
      >
        <p className="totp-warning">
          关闭后账号将只使用密码登录，安全性会降低。请输入当前密码，并输入动态验证码或任一恢复码确认。
        </p>
        <Form form={disableForm} layout="vertical" onFinish={(values) => disableMutation.mutate(values)}>
          <Form.Item name="password" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
            <Input.Password placeholder="当前密码" autoComplete="current-password" />
          </Form.Item>
          <Form.Item name="otp_code" label="动态验证码或恢复码" rules={[{ required: true, message: '请输入验证码' }]}>
            <Input placeholder="000000 或 XXXX-XXXX" maxLength={9} autoComplete="one-time-code" />
          </Form.Item>
          <div className="totp-modal-actions">
            <Button
              disabled={disableMutation.isPending}
              onClick={() => {
                setDisableOpen(false)
                disableForm.resetFields()
              }}
            >
              取消
            </Button>
            <Button danger type="primary" htmlType="submit" loading={disableMutation.isPending}>
              确认关闭
            </Button>
          </div>
        </Form>
      </Modal>

      {/* 添加通行密钥：命名设备并验证当前密码。 */}
      <Modal
        open={passkeyOpen}
        footer={null}
        width={520}
        title="添加通行密钥"
        onCancel={closePasskey}
      >
        <p className="totp-hint">
          为这台设备起个名字。点击「开始验证」后，浏览器会调用指纹、面容或设备 PIN 完成绑定。
        </p>
        <Form form={passkeyForm} layout="vertical" onFinish={(values) => addPasskeyMutation.mutate(values)}>
          <Form.Item
            name="name"
            label="设备名称"
            rules={[{ required: true, message: '请输入设备名称' }]}
            initialValue="我的设备"
          >
            <Input maxLength={64} placeholder="例如：MacBook Pro" />
          </Form.Item>
          <Form.Item
            name="password"
            label="当前密码"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password placeholder="确认是你本人操作" autoComplete="current-password" />
          </Form.Item>
          <div className="totp-modal-actions">
            <Button disabled={addPasskeyMutation.isPending} onClick={closePasskey}>
              取消
            </Button>
            <Button type="primary" htmlType="submit" loading={addPasskeyMutation.isPending}>
              开始验证
            </Button>
          </div>
        </Form>
      </Modal>

      {/* 重命名已绑定的通行密钥。 */}
      <Modal
        open={!!renameTarget}
        footer={null}
        width={420}
        title="重命名通行密钥"
        onCancel={() => {
          if (renamePasskeyMutation.isPending) return
          setRenameTarget(null)
          renameForm.resetFields()
        }}
      >
        <Form
          form={renameForm}
          layout="vertical"
          onFinish={(values) => {
            if (!renameTarget) return
            renamePasskeyMutation.mutate({ id: renameTarget.id, name: values.name })
          }}
        >
          <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '请输入设备名称' }]}>
            <Input maxLength={64} placeholder="例如：iPhone" />
          </Form.Item>
          <div className="totp-modal-actions">
            <Button
              disabled={renamePasskeyMutation.isPending}
              onClick={() => {
                setRenameTarget(null)
                renameForm.resetFields()
              }}
            >
              取消
            </Button>
            <Button type="primary" htmlType="submit" loading={renamePasskeyMutation.isPending}>
              保存名称
            </Button>
          </div>
        </Form>
      </Modal>
    </>
  )
}
