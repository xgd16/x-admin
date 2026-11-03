import dayjs from 'dayjs'

export function formatDate(value?: string | null): string {
  if (!value) return '—'
  return dayjs(value).format('YYYY-MM-DD HH:mm')
}

export function formatSize(value: number): string {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

export function flattenTree<T extends { children?: T[] }>(rows: T[]): T[] {
  const out: T[] = []
  const walk = (nodes: T[]) => {
    nodes.forEach((node) => {
      out.push(node)
      if (node.children?.length) walk(node.children)
    })
  }
  walk(rows)
  return out
}

export function initials(name?: string): string {
  return (name ?? 'X').slice(0, 1).toUpperCase()
}
