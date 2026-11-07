/**
 * 全局 Toast 提示：统一封装 antd message / notification 的位置、时长与关闭方式。
 * 页面只负责描述结果，样式与交互由 ThemeProvider 和 `_toast.scss` 统一维护。
 */
import { App } from 'antd'
import { useMemo, type ReactNode } from 'react'

/** antd message 只接受 string | number 作为 key。 */
type ToastKey = string | number

/** 轻提示的默认展示时长（秒）；错误信息停留更久，方便阅读。 */
const DURATION = {
  success: 2.4,
  info: 2.8,
  warning: 3.6,
  error: 4.5,
  notification: 4.5,
} as const

/** 轻提示的可选参数，key 用于原地更新同一条提示。 */
export interface ToastOptions {
  key?: ToastKey
  duration?: number
  onClick?: () => void
}

/** 通知卡片参数，适合展示多行结果或需要用户确认的信息。 */
export interface ToastNotifyOptions {
  type?: 'info' | 'success' | 'warning' | 'error'
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  key?: ToastKey
  duration?: number
}

/** 从任意异常中提取可展示的文案，避免页面里重复写三元表达式。 */
export function toErrorMessage(error: unknown, fallback = '操作失败') {
  if (typeof error === 'string' && error.trim()) return error
  if (error instanceof Error && error.message) return error.message
  if (typeof error === 'object' && error !== null) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) return message
  }
  return fallback
}

/** 生成不重复的 key，防止并发请求的同名 loading 互相覆盖。 */
let toastSequence = 0

export function useToast() {
  const { message, notification } = App.useApp()

  return useMemo(() => {
    const open = (type: 'success' | 'info' | 'warning' | 'error', content: ReactNode, options?: ToastOptions) => {
      message.open({
        type,
        content,
        key: options?.key,
        duration: options?.duration ?? DURATION[type],
        onClick: options?.onClick,
      })
    }

    return {
      success(content: ReactNode, options?: ToastOptions) {
        open('success', content, options)
      },
      info(content: ReactNode, options?: ToastOptions) {
        open('info', content, options)
      },
      warning(content: ReactNode, options?: ToastOptions) {
        open('warning', content, options)
      },
      error(content: ReactNode, options?: ToastOptions) {
        open('error', content, options)
      },
      /** 统一处理 catch 到的异常，保留完整错误文案。 */
      fail(error: unknown, fallback?: string) {
        open('error', toErrorMessage(error, fallback))
      },
      /** 展示 loading，返回关闭函数；需要更新文案时请传 key。 */
      loading(content: ReactNode, options?: ToastOptions) {
        return message.loading({
          content,
          key: options?.key,
          duration: 0,
          onClick: options?.onClick,
        })
      },
      /** loading → success / error 的完整流程，常用于下载、导入等长请求。 */
      async promise<T>(
        task: Promise<T>,
        texts: {
          loading: ReactNode
          success: ReactNode | ((value: T) => ReactNode)
          error?: ReactNode | ((error: unknown) => ReactNode)
          key?: ToastKey
        },
      ) {
        const key = texts.key ?? `toast-promise-${++toastSequence}`
        message.loading({ content: texts.loading, key, duration: 0 })
        try {
          const value = await task
          message.success({
            content: typeof texts.success === 'function' ? texts.success(value) : texts.success,
            key,
            duration: DURATION.success,
          })
          return value
        } catch (error) {
          message.error({
            content:
              typeof texts.error === 'function'
                ? texts.error(error)
                : texts.error ?? toErrorMessage(error),
            key,
            duration: DURATION.error,
          })
          throw error
        }
      },
      /** 通知卡片：适合导入结果、批量操作汇总等需要展开说明的场景。 */
      notify(options: ToastNotifyOptions) {
        notification.open({
          type: options.type ?? 'info',
          title: options.title,
          description: options.description,
          actions: options.actions,
          key: options.key,
          duration: options.duration ?? DURATION.notification,
        })
      },
      dismiss(key?: ToastKey) {
        message.destroy(key)
      },
    }
  }, [message, notification])
}
