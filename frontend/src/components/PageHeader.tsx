/**
 * 页面标题与操作区。
 */
import type { ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  sub?: ReactNode
  actions?: ReactNode
}

/** 页面顶部标题区，统一标题、副标题和右侧操作按钮的布局。 */
export function PageHeader({ title, sub, actions }: PageHeaderProps) {
  return (
    <header className="page-head">
      <div>
        <h1>{title}</h1>
        {sub && <p className="sub">{sub}</p>}
      </div>
      {actions && <div className="head-actions">{actions}</div>}
    </header>
  )
}
