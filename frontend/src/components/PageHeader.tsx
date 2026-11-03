import type { ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  sub?: ReactNode
  actions?: ReactNode
}

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
