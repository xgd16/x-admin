/**
 * 统一加载态：应用启动、行内提示、表格/列表/图表骨架屏。
 *
 * 视觉都交给 CSS 变量与 antd 的 Spin / Skeleton，页面只选择“用哪种加载形态”：
 * - PageLoader：冷启动/会话恢复，品牌级启动动画
 * - LoadingIndicator：表格刷新、按钮内联等小尺寸指示器
 * - InlineLoader：局部区域的一句话加载提示
 * - ListSkeleton / TableSkeleton / ChartSkeleton / FileGridSkeleton：首次加载的骨架屏
 */
import type { CSSProperties, ReactNode } from 'react'
import { Skeleton, Spin } from 'antd'
import { BrandMark } from './BrandMark'

type LoaderSize = 'sm' | 'md' | 'lg'

const INDICATOR_SIZE: Record<LoaderSize, number> = { sm: 16, md: 24, lg: 36 }

interface LoadingIndicatorProps {
  size?: LoaderSize
  /** 深色/彩色背景上使用白色描边。 */
  inverse?: boolean
}

/** 环形加载指示器：暖色渐变弧 + 轨道光点。 */
export function LoadingIndicator({ size = 'md', inverse = false }: LoadingIndicatorProps) {
  return (
    <span
      className={`x-loader${inverse ? ' is-inverse' : ''}`}
      style={{ '--x-loader-size': `${INDICATOR_SIZE[size]}px` } as CSSProperties}
      aria-hidden="true"
    >
      <span className="x-loader-halo" />
      <span className="x-loader-ring" />
      <span className="x-loader-dot" />
    </span>
  )
}

/** 启动页中央的品牌加载标记。 */
export function BrandLoader() {
  return (
    <span className="x-brand-loader" aria-hidden="true">
      <span className="x-brand-loader-halo" />
      <span className="x-brand-loader-pulse" />
      <span className="x-brand-loader-pulse is-late" />
      <span className="x-brand-loader-orbit" />
      <span className="x-brand-loader-logo">
        <BrandMark size={26} />
      </span>
    </span>
  )
}

interface PageLoaderProps {
  label?: string
  hint?: string
}

/** 全屏启动加载：用于恢复登录态、首次进入应用等场景。 */
export function PageLoader({
  label = '正在恢复工作区',
  hint = '同步账号权限 · 导航菜单 · 工作台配置',
}: PageLoaderProps) {
  return (
    <div className="x-page-loader" role="status" aria-live="polite">
      <span className="x-page-loader-aurora" aria-hidden="true" />
      <div className="x-page-loader-card glass">
        <Spin indicator={<BrandLoader />} />
        <strong className="x-page-loader-name">X-ADMIN</strong>
        <span className="x-page-loader-label">{label}</span>
        <span className="x-page-loader-track" aria-hidden="true">
          <i />
        </span>
        <span className="x-page-loader-hint">{hint}</span>
      </div>
    </div>
  )
}

interface InlineLoaderProps {
  label?: ReactNode
  size?: LoaderSize
  /** 占满整行并居中，适合弹窗、空状态区域。 */
  block?: boolean
  className?: string
}

/** 局部加载提示：小尺寸指示器 + 一句说明。 */
export function InlineLoader({ label = '正在加载', size = 'sm', block = false, className }: InlineLoaderProps) {
  return (
    <div
      className={`x-inline-loader${block ? ' is-block' : ''}${className ? ` ${className}` : ''}`}
      role="status"
      aria-live="polite"
    >
      <Spin size="small" indicator={<LoadingIndicator size={size} />} />
      {label && <span>{label}</span>}
    </div>
  )
}

interface ListSkeletonProps {
  rows?: number
  avatar?: boolean
  compact?: boolean
  className?: string
}

/** 列表骨架：通知、设备、分享记录等纵向条目。 */
export function ListSkeleton({ rows = 3, avatar = true, compact = false, className }: ListSkeletonProps) {
  return (
    <div
      className={`x-list-skeleton${compact ? ' is-compact' : ''}${className ? ` ${className}` : ''}`}
      role="status"
      aria-label="正在加载列表"
    >
      {Array.from({ length: rows }).map((_, index) => (
        <div
          className="x-list-skeleton-row"
          key={index}
          style={{ '--x-skeleton-delay': `${index * 70}ms` } as CSSProperties}
        >
          <Skeleton
            active
            avatar={avatar ? { size: 34, shape: 'circle' } : false}
            title={{ width: `${44 + (index % 3) * 9}%` }}
            paragraph={{ rows: 1, width: ['84%'] }}
          />
        </div>
      ))}
    </div>
  )
}

interface TableSkeletonProps {
  rows?: number
  columns?: number[]
}

/** 表格骨架：首屏数据未到达时填充表体，保留真实表头。 */
export function TableSkeleton({ rows = 5, columns = [22, 16, 14, 18, 12] }: TableSkeletonProps) {
  return (
    <div className="x-table-skeleton" role="status" aria-label="正在加载表格数据">
      {Array.from({ length: rows }).map((_, rowIndex) => (
        <div className="x-table-skeleton-row" key={rowIndex}>
          {columns.map((width, columnIndex) => (
            <div
              key={columnIndex}
              className="x-table-skeleton-cell"
              style={{ width: `${Math.max(8, width + ((rowIndex + columnIndex) % 3) - 1)}%` }}
            >
              <Skeleton.Node active style={{ width: '100%', height: 10 }} />
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}

interface MetricSkeletonProps {
  width?: number
}

/** 指标数值骨架：用于 KPI 大数字等单值区域。 */
export function MetricSkeleton({ width = 72 }: MetricSkeletonProps) {
  return <Skeleton.Node active className="x-metric-skeleton" style={{ width, height: 30 }} />
}

/** 图表骨架：保留面积图区域的呼吸感，并说明正在绘制。 */
export function ChartSkeleton() {
  return (
    <div className="chart-wrap" aria-hidden="true">
      <div className="chart chart-skeleton x-chart-loading">
        <span className="x-chart-loading-hint">
          <LoadingIndicator size="sm" />
          <span>正在绘制趋势</span>
        </span>
      </div>
    </div>
  )
}

interface TreeSkeletonProps {
  rows?: number
}

/** 树形骨架：菜单、部门等层级数据的首屏加载。 */
export function TreeSkeleton({ rows = 6 }: TreeSkeletonProps) {
  const indents = [0, 0, 22, 44, 22, 0]
  return (
    <div className="x-tree-skeleton" role="status" aria-label="正在加载树形数据">
      {Array.from({ length: rows }).map((_, index) => (
        <div
          className="x-tree-skeleton-row"
          key={index}
          style={{
            paddingLeft: indents[index % indents.length],
            '--x-skeleton-delay': `${index * 55}ms`,
          } as CSSProperties}
        >
          <Skeleton.Node active style={{ width: 18, height: 18 }} />
          <div className="x-tree-skeleton-line" style={{ width: `${42 + (index % 3) * 11}%` }}>
            <Skeleton.Node active style={{ width: '100%', height: 10 }} />
          </div>
          <div className="x-tree-skeleton-line is-short" style={{ width: `${18 + (index % 2) * 8}%` }}>
            <Skeleton.Node active style={{ width: '100%', height: 8 }} />
          </div>
        </div>
      ))}
    </div>
  )
}

interface FileGridSkeletonProps {
  count?: number
}

/** 文件库卡片骨架：文件选择弹窗首屏加载。 */
export function FileGridSkeleton({ count = 8 }: FileGridSkeletonProps) {
  return (
    <div className="x-file-grid-skeleton" role="status" aria-label="正在加载文件库">
      {Array.from({ length: count }).map((_, index) => (
        <div
          className="x-file-grid-skeleton-card"
          key={index}
          style={{ '--x-skeleton-delay': `${index * 45}ms` } as CSSProperties}
        >
          <Skeleton.Image active className="x-file-grid-skeleton-thumb" />
          <div className="x-file-grid-skeleton-line" style={{ width: `${68 + (index % 3) * 8}%` }}>
            <Skeleton.Node active style={{ width: '100%', height: 7 }} />
          </div>
          <div
            className="x-file-grid-skeleton-line is-short"
            style={{ width: `${38 + (index % 4) * 6}%` }}
          >
            <Skeleton.Node active style={{ width: '100%', height: 6 }} />
          </div>
        </div>
      ))}
    </div>
  )
}
