import { Tag } from 'antd'

export function StatusTag({ status, on, off, colorOn = 'green' }: { status: 0 | 1; on?: string; off?: string; colorOn?: string }) {
  if (status === 1) return <Tag color={colorOn}>{on ?? '正常'}</Tag>
  return <Tag>{off ?? '停用'}</Tag>
}

export function MethodTag({ method }: { method: string }) {
  const color =
    method === 'GET' ? 'blue' : method === 'POST' ? 'green' : method === 'PUT' || method === 'PATCH' ? 'orange' : 'red'
  return <Tag color={color}>{method}</Tag>
}
