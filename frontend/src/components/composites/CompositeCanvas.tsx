import { useMemo } from 'react'

import { useT } from '../../i18n'
import type { ComponentModelRef, CrossModelJoin } from '../../types/composite'

interface CompositeCanvasProps {
  components: ComponentModelRef[]
  crossJoins: CrossModelJoin[]
  modelNames?: Record<string, string>
}

interface NodeBox {
  alias: string
  label: string
  role: string
  x: number
  y: number
  width: number
  height: number
}

const NODE_W = 160
const NODE_H = 70
const GAP_X = 90
const GAP_Y = 60
const PAD = 24

const PALETTE = [
  '#2563eb',
  '#16a34a',
  '#d97706',
  '#9333ea',
  '#dc2626',
  '#0891b2',
  '#ca8a04',
  '#db2777',
]

// CompositeCanvas renders component models as nodes and cross-model joins as
// connecting lines, giving a domain-colored overview of the merged model.
export function CompositeCanvas({ components, crossJoins, modelNames }: CompositeCanvasProps) {
  const t = useT()
  const { nodes, width, height } = useMemo(() => {
    const perRow = Math.max(1, Math.ceil(Math.sqrt(components.length)))
    const boxes: NodeBox[] = components.map((c, i) => {
      const col = i % perRow
      const row = Math.floor(i / perRow)
      return {
        alias: c.alias,
        label: modelNames?.[c.model_id] ?? c.alias,
        role: c.role,
        x: PAD + col * (NODE_W + GAP_X),
        y: PAD + row * (NODE_H + GAP_Y),
        width: NODE_W,
        height: NODE_H,
      }
    })
    const rows = Math.ceil(components.length / perRow)
    return {
      nodes: boxes,
      width: PAD * 2 + perRow * NODE_W + (perRow - 1) * GAP_X,
      height: PAD * 2 + rows * NODE_H + (rows - 1) * GAP_Y,
    }
  }, [components, modelNames])

  const byAlias = useMemo(() => {
    const m: Record<string, NodeBox> = {}
    for (const n of nodes) {
      m[n.alias] = n
    }
    return m
  }, [nodes])

  const colorFor = (alias: string) => {
    const idx = nodes.findIndex((n) => n.alias === alias)
    return PALETTE[idx % PALETTE.length] ?? '#2563eb'
  }

  if (components.length === 0) {
    return <div className="composite-canvas-empty">{t('composites.canvas_empty')}</div>
  }

  return (
    <svg
      className="composite-canvas"
      width="100%"
      viewBox={`0 0 ${Math.max(width, 320)} ${Math.max(height, 140)}`}
      role="img"
      aria-label={t('composites.canvas_aria')}
    >
      {crossJoins.map((j, i) => {
        const from = byAlias[j.from_model]
        const to = byAlias[j.to_model]
        if (!from || !to) {
          return null
        }
        const x1 = from.x + from.width / 2
        const y1 = from.y + from.height / 2
        const x2 = to.x + to.width / 2
        const y2 = to.y + to.height / 2
        const midX = (x1 + x2) / 2
        const midY = (y1 + y2) / 2
        return (
          <g key={j.id ?? `cj-${i}`}>
            <line
              x1={x1}
              y1={y1}
              x2={x2}
              y2={y2}
              stroke="#94a3b8"
              strokeWidth={1.5}
              strokeDasharray="4 3"
            />
            <text x={midX} y={midY - 4} textAnchor="middle" fontSize={10} fill="#64748b">
              {j.relationship}
            </text>
          </g>
        )
      })}
      {nodes.map((n) => {
        const color = colorFor(n.alias)
        return (
          <g key={n.alias}>
            <rect
              x={n.x}
              y={n.y}
              width={n.width}
              height={n.height}
              rx={10}
              fill={`${color}14`}
              stroke={color}
              strokeWidth={n.role === 'primary' ? 2.5 : 1.5}
            />
            <text
              x={n.x + n.width / 2}
              y={n.y + 26}
              textAnchor="middle"
              fontSize={13}
              fontWeight={600}
              fill={color}
            >
              {n.label.length > 18 ? `${n.label.slice(0, 17)}…` : n.label}
            </text>
            <text
              x={n.x + n.width / 2}
              y={n.y + 44}
              textAnchor="middle"
              fontSize={11}
              fill="#64748b"
            >
              {n.alias}
            </text>
            <text
              x={n.x + n.width / 2}
              y={n.y + 60}
              textAnchor="middle"
              fontSize={10}
              fill="#94a3b8"
            >
              {n.role}
            </text>
          </g>
        )
      })}
    </svg>
  )
}
