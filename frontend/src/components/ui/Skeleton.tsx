import clsx from 'clsx'
import type { CSSProperties } from 'react'

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
      className={clsx(
        "relative block h-4 w-full overflow-hidden rounded-[0.4rem] bg-(--bg-card-raised,rgba(148,163,184,0.12)) after:absolute after:inset-0 after:-translate-x-full after:animate-[skeleton-shimmer_1.3s_ease-in-out_infinite] after:bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.08),transparent)] after:content-[''] motion-reduce:after:animate-none",
        circle && 'rounded-full!',
        className,
      )}
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
    <span className={clsx('grid gap-2', className)} aria-hidden="true">
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
      className={clsx('grid gap-[0.6rem]', className)}
      style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }}
      aria-hidden="true"
    >
      {Array.from({ length: rows * columns }, (_, i) => (
        <Skeleton key={i} height="1rem" />
      ))}
    </div>
  )
}
