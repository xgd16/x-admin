import type { ReactNode } from 'react'
import { Form, Modal } from 'antd'
import type { FormInstance } from 'antd'

interface FormModalProps {
  open: boolean
  title: string
  form: FormInstance
  onCancel: () => void
  onSubmit: (values: Record<string, unknown>) => void | Promise<void>
  children: ReactNode
  width?: number
  confirmLoading?: boolean
  okText?: string
}

export function FormModal({
  open,
  title,
  form,
  onCancel,
  onSubmit,
  children,
  width = 520,
  confirmLoading,
  okText = '保存',
}: FormModalProps) {
  return (
    <Modal
      open={open}
      title={title}
      width={width}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={confirmLoading}
      okText={okText}
      cancelText="取消"
      destroyOnHidden
      mask={{ closable: false }}
    >
      <Form form={form} layout="vertical" onFinish={onSubmit} requiredMark="optional" style={{ marginTop: 12 }}>
        {children}
      </Form>
    </Modal>
  )
}
