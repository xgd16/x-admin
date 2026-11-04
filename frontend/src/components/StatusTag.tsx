/**
 * 状态与 HTTP 方法标签。
 */
import { Tag } from 'antd'

/** 通用启用/停用标签，文案与颜色可按业务覆盖。 */
export function StatusTag({ status, on, off, colorOn = 'green' }: { status: 0 | 1; on?: string; off?: string; colorOn?: string }) {
  if (status === 1) return <Tag color={colorOn}>{on ?? '正常'}</Tag>
  return <Tag>{off ?? '停用'}</Tag>
}

/** HTTP 方法标签：GET 蓝、POST 绿、写操作橙、删除类红色。 */
export function MethodTag({ method }: { method: string }) {
  const color =
    method === 'GET' ? 'blue' : method === 'POST' ? 'green' : method === 'PUT' || method === 'PATCH' ? 'orange' : 'red'
  return <Tag color={color}>{method}</Tag>
}
