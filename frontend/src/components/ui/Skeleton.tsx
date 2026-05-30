import clsx from 'clsx'
import type { CSSProperties } from 'react'
import '../../styles/skeleton.css'

interface SkeletonProps {
  width?: number | string
  height?: number | string
  radius?: number | string
  circle?: boolean
  className?: string
  style?: CSSProperties
}

/** A single shimmering placeholder block. Decorative — hidden from assistive tech. */
export function Skeleton({ width, height, radius, circle, className, style }: SkeletonProps) {
  return (
    <span
      className={clsx('skeleton', circle && 'skeleton--circle', className)}
      aria-hidden="true"
      style={{
        width,
        height,
        borderRadius: circle ? '50%' : radius,
        ...style,
      }}
    />
  )
}

interface SkeletonTextProps {
  lines?: number
  className?: string
}

/** A stack of text-line placeholders; the last line is shortened. */
export function SkeletonText({ lines = 3, className }: SkeletonTextProps) {
  return (
    <span className={clsx('skeleton-text', className)} aria-hidden="true">
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} height="0.8em" width={i === lines - 1 ? '60%' : '100%'} />
      ))}
    </span>
  )
}

interface SkeletonTableProps {
  rows?: number
  columns?: number
  className?: string
}

/** A grid of placeholder cells approximating a loading data table. */
export function SkeletonTable({ rows = 5, columns = 4, className }: SkeletonTableProps) {
  return (
    <div
      className={clsx('skeleton-table', className)}
      style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }}
      aria-hidden="true"
    >
      {Array.from({ length: rows * columns }, (_, i) => (
        <Skeleton key={i} height="1rem" />
      ))}
    </div>
  )
}
