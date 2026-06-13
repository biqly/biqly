import { useId } from 'react'

import { qbJoinTypeIconClass } from '../queryBuilder/queryBuilderClasses'
import { normalizeJoinType } from './joinType'

/** Venn diagram for a SQL join type: filled regions show which rows survive. */
export function JoinTypeIcon({ type, size = 18 }: { type: string; size?: number }) {
  const clipId = useId()
  const normalized = normalizeJoinType(type) ?? 'inner'
  const fillLeft = normalized === 'left' || normalized === 'full'
  const fillRight = normalized === 'right' || normalized === 'full'

  return (
    <svg
      width={size}
      height={(size * 2) / 3}
      viewBox="0 0 24 16"
      aria-hidden="true"
      focusable="false"
      className={qbJoinTypeIconClass}
    >
      <defs>
        <clipPath id={clipId}>
          <circle cx="9.5" cy="8" r="6.5" />
        </clipPath>
      </defs>
      <circle
        cx="9.5"
        cy="8"
        r="6.5"
        fill={fillLeft ? 'currentColor' : 'none'}
        fillOpacity={fillLeft ? 0.85 : 0}
        stroke="currentColor"
        strokeWidth="1.4"
      />
      <circle
        cx="14.5"
        cy="8"
        r="6.5"
        fill={fillRight ? 'currentColor' : 'none'}
        fillOpacity={fillRight ? 0.85 : 0}
        stroke="currentColor"
        strokeWidth="1.4"
      />
      <circle
        cx="14.5"
        cy="8"
        r="6.5"
        fill="currentColor"
        fillOpacity="0.85"
        clipPath={`url(#${clipId})`}
      />
    </svg>
  )
}
