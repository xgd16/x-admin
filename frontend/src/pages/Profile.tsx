import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Button, Form, Input } from 'antd'
import { api } from '../api/client'
import type { User } from '../api/types'
import { useAuth } from '../stores/auth'
import { PageHeader } from '../components/PageHeader'
import { FileSelect } from '../components/FileSelect'
import { Icon } from '../components/Icon'
import { formatDate, initials } from '../utils/format'

interface ProfileValues {
  nickname: string
  email: string
  phone: string
  avatar: string
}

export function Profile() {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const storeUser = useAuth((state) => state.user)
  const can = useAuth((state) => state.can)
  const canUseFileLibrary = can('system:file:list')
  const [profileForm] = Form.useForm<ProfileValues>()
  const [passwordForm] = Form.useForm<{ old_password: string; new_password: string; confirm: string }>()

  const query = useQuery({
    queryKey: ['profile'],
    queryFn: () => api.get<User>('/profile'),
    staleTime: 30_000,
  })

  const user = query.data ?? storeUser

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
    </>
  )
}
