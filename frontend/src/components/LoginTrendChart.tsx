import { useMemo, useState, type MouseEvent as ReactMouseEvent } from 'react'
import dayjs from 'dayjs'
import type { TrendPoint } from '../api/types'
import { CHART, CHART_PLOT_WIDTH, nearestTrendIndex, smoothPath, tooltipAnchor, trendPoints } from '../utils/chart'

const numberFormat = new Intl.NumberFormat('zh-CN')

interface LoginTrendChartProps {
  trend: TrendPoint[]
}

export function LoginTrendChart({ trend }: LoginTrendChartProps) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)
  const points = useMemo(() => trendPoints(trend), [trend])
  const line = useMemo(() => smoothPath(points), [points])
  const area = points.length
    ? `${line} L${points[points.length - 1].x} ${CHART.baseline} L${points[0].x} ${CHART.baseline} Z`
    : ''
  const peakIndex = useMemo(
    () => trend.reduce((best, point, index, all) => (point.count > all[best].count ? index : best), 0),
    [trend],
  )
  const peak = trend[peakIndex]
  const activeIndex = hoverIndex ?? (peak && peak.count > 0 ? peakIndex : null)
  const active = activeIndex === null ? undefined : trend[activeIndex]
  const activePoint = activeIndex === null ? undefined : points[activeIndex]
  const anchor = activePoint ? tooltipAnchor(activePoint) : null
  const showTooltip = Boolean(active && activePoint && anchor && (hoverIndex !== null || active.count > 0))

  const handleMove = (event: ReactMouseEvent<SVGRectElement>) => {
    const svg = event.currentTarget.ownerSVGElement
    if (!svg) return
    setHoverIndex(nearestTrendIndex(event.clientX, svg.getBoundingClientRect(), trend.length))
  }

  return (
    <div className="chart-wrap">
      <svg className="chart" viewBox={`0 0 ${CHART.width} ${CHART.height}`} fill="none" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id="login-line" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="#ff9a6c" />
            <stop offset="55%" stopColor="#ff5f33" />
            <stop offset="100%" stopColor="#ff4d8d" />
          </linearGradient>
          <linearGradient id="login-area" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#ff6a3d" stopOpacity=".22" />
            <stop offset="100%" stopColor="#ff6a3d" stopOpacity="0" />
          </linearGradient>
          <filter id="login-glow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="2.5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>
        <g stroke="var(--chart-grid)" strokeWidth="1">
          <path d="M42 30h700M42 70h700M42 110h700M42 150h700M42 190h700" />
        </g>
        <g fill="var(--chart-axis)" fontSize="8" fontFamily="monospace">
          <text x="4" y="33">100%</text>
          <text x="4" y="73">75%</text>
          <text x="4" y="113">50%</text>
          <text x="4" y="153">25%</text>
          <text x="4" y="193">0</text>
        </g>
        {area && <path d={area} fill="url(#login-area)" />}
        {line && (
          <path
            d={line}
            stroke="url(#login-line)"
            strokeWidth="2.4"
            strokeLinecap="round"
            strokeLinejoin="round"
            filter="url(#login-glow)"
          />
        )}
        {points.map((point, index) => (
          <circle
            key={`${point.x}-${index}`}
            cx={point.x}
            cy={point.y}
            r="2.6"
            fill="var(--chart-dot)"
            stroke="#ff5f33"
            strokeWidth="1.6"
          />
        ))}
        {activePoint && (
          <g pointerEvents="none">
            <line
              x1={activePoint.x}
              y1={CHART.top}
              x2={activePoint.x}
              y2={CHART.baseline}
              stroke="rgba(255,95,51,.3)"
              strokeWidth="1"
              strokeDasharray="3 4"
            />
            <circle cx={activePoint.x} cy={activePoint.y} r="6.4" fill="rgba(255,95,51,.16)" />
            <circle cx={activePoint.x} cy={activePoint.y} r="3.1" fill="#ff5f33" stroke="#fff" strokeWidth="1.4" />
          </g>
        )}
        {trend.length > 0 && (
          <rect
            className="chart-hit"
            x={CHART.left}
            y={CHART.top}
            width={CHART_PLOT_WIDTH}
            height={CHART.baseline - CHART.top}
            fill="transparent"
            pointerEvents="all"
            onMouseMove={handleMove}
            onMouseLeave={() => setHoverIndex(null)}
          />
        )}
      </svg>
      {showTooltip && active && activePoint && anchor && (
        <div
          className={`chart-tooltip align-${anchor.align}`}
          style={{
            left: `${anchor.left}%`,
            top: `${anchor.top}%`,
            transform: `translate(${anchor.align === 'center' ? '-50%' : anchor.align === 'right' ? '-100%' : '0'}, ${
              anchor.below ? '12px' : 'calc(-100% - 12px)'
            })`,
          }}
        >
          <b>{numberFormat.format(active.count)} 次</b>
          {dayjs(active.date).format('MM-DD')} · {activeIndex === peakIndex && active.count > 0 ? '峰值' : '当日'}
        </div>
      )}
    </div>
  )
}
