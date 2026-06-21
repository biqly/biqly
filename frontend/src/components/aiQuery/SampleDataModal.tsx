import { useEffect, useMemo, useState } from 'react'

import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formatResultCell } from '../../utils/resultCellFormat'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Modal } from '../ui/Modal'
import type { SampleData } from './types'

function parseTableRef(tableName: string): { schema: string; table: string } {
  const parts = tableName.split('.')
  if (parts.length >= 2) {
    return { schema: parts[0] ?? 'public', table: parts.slice(1).join('.') }
  }
  return { schema: 'public', table: tableName }
}

type SampleColumnKind = 'id' | 'text' | 'handle' | 'default'

function sampleColumnKind(name: string): SampleColumnKind {
  const u = name.toUpperCase()
  if (u === 'ID' || u.endsWith('_ID')) {
    return 'id'
  }
  if (
    u === 'SCREEN_NAME' ||
    u === 'USERNAME' ||
    u === 'USER_NAME' ||
    u.endsWith('_HANDLE') ||
    (u.endsWith('_NAME') && u !== 'NAME')
  ) {
    return 'handle'
  }
  if (
    u === 'TEXT' ||
    u === 'BODY' ||
    u === 'CONTENT' ||
    u === 'DESCRIPTION' ||
    u.includes('TEXT') ||
    u.includes('MESSAGE') ||
    u.includes('COMMENT')
  ) {
    return 'text'
  }
  return 'default'
}

function columnClass(kind: SampleColumnKind): string {
  if (kind === 'id') {
    return 'w-44 max-w-[11rem]'
  }
  if (kind === 'handle') {
    return 'w-36 max-w-[9rem] whitespace-nowrap'
  }
  if (kind === 'text') {
    return 'min-w-[22rem] max-w-[36rem] max-[720px]:min-w-[14rem] max-[720px]:max-w-[20rem]'
  }
  return ''
}

function formatSampleScalar(value: unknown, columnName: string): string {
  if (value === null || value === undefined) {
    return ''
  }
  if (sampleColumnKind(columnName) === 'id') {
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'bigint') {
      return String(value)
    }
    if (typeof value === 'boolean') {
      return value ? 'true' : 'false'
    }
    return JSON.stringify(value)
  }
  return formatResultCell(value, columnName, {})
}

function SampleCell({ value, columnName }: { value: unknown; columnName: string }) {
  const text = formatSampleScalar(value, columnName)
  const kind = sampleColumnKind(columnName)

  if (!text) {
    return (
      <span className="text-foreground block leading-[1.45] wrap-break-word opacity-55">—</span>
    )
  }

  if (kind === 'handle') {
    const handle = text.startsWith('@') ? text : `@${text}`
    return (
      <span className="text-accent inline-flex max-w-full items-center overflow-hidden rounded-full border border-[color-mix(in_srgb,var(--accent)_22%,var(--border))] bg-[color-mix(in_srgb,var(--accent)_6%,transparent)] px-[0.45rem] py-[0.15rem] font-mono text-[0.78rem] font-semibold text-ellipsis whitespace-nowrap">
        {handle}
      </span>
    )
  }

  if (kind === 'id') {
    return (
      <span className="text-foreground-muted font-mono text-[0.78rem] break-all [font-variant-numeric:tabular-nums]">
        {text}
      </span>
    )
  }

  if (kind === 'text') {
    return (
      <span
        className="text-foreground-muted group-hover/row:text-foreground line-clamp-4 overflow-hidden text-[0.84rem]"
        title={text}
      >
        {text}
      </span>
    )
  }

  return <span className="text-foreground block leading-[1.45] wrap-break-word">{text}</span>
}

export function SampleDataModal({
  open,
  onClose,
  tableName,
  datasourceId,
  get,
}: {
  open: boolean
  onClose: () => void
  tableName: string
  datasourceId: string
  get: <T>(url: string) => Promise<T | null>
}) {
  const t = useT()
  const [sample, setSample] = useState<SampleData | null>(null)
  const [loading, setLoading] = useState(false)

  const tableRef = useMemo(() => parseTableRef(tableName), [tableName])

  useEffect(() => {
    if (!open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSample(null)
      return
    }
    setLoading(true)
    const [schema, ...rest] = tableName.split('.')
    const tName = rest.length > 0 ? rest.join('.') : schema
    const url = `/api/datasources/${datasourceId}/tables/${schema ?? 'public'}/${tName}/sample`
    void get<SampleData>(url).then((data) => {
      setSample(data)
      setLoading(false)
    })
  }, [datasourceId, get, open, tableName])

  const rowCount = sample?.rows.length ?? 0
  const colCount = sample?.columns.length ?? 0

  return (
    <Modal
      open={open}
      title={t('ai_query.sample_modal_heading')}
      subtitle={
        <span className="mt-[0.35rem] inline-flex flex-wrap items-baseline gap-0 font-mono text-[0.82rem] leading-[1.35]">
          <span className="text-accent font-semibold">{tableRef.schema}</span>
          <span className="text-foreground-muted opacity-70" aria-hidden="true">
            .
          </span>
          <span className="text-foreground-muted font-medium">{tableRef.table}</span>
        </span>
      }
      onClose={onClose}
      labelledBy="sample-data-title"
      className="flex max-h-[min(88vh,52rem)]! w-[min(96vw,72rem)]! flex-col max-[720px]:max-h-[92vh]! max-[720px]:w-[min(100%,100vw-1rem)]!"
      bodyClassName="!flex !flex-col gap-3 p-[0.85rem_1.1rem_1.1rem] max-[720px]:p-[0.75rem_0.85rem_0.9rem] min-h-0 flex-1"
    >
      <LoadingOverlay loading={loading} />
      {!loading && sample && rowCount > 0 && (
        <>
          <div className="flex items-center justify-between gap-3" aria-live="polite">
            <span
              className={
                'text-foreground-muted inline-flex items-center rounded-full border border-[color-mix(in_srgb,var(--accent)_28%,var(--border))] bg-[color-mix(in_srgb,var(--accent)_8%,var(--bg-card-raised))] px-[0.65rem] py-[0.28rem] text-[0.76rem] font-semibold tracking-wide'
              }
            >
              {t('ai_query.sample_modal_meta', { rows: rowCount, cols: colCount })}
            </span>
          </div>
          <div
            className={
              'border-border bg-card-raised custom-scrollbar-thin max-h-[min(62vh,40rem)] min-h-0 flex-1 overflow-auto overscroll-contain rounded-[0.65rem] border shadow-[inset_0_1px_0_color-mix(in_srgb,var(--text-primary)_4%,transparent)] max-[720px]:max-h-[58vh]'
            }
          >
            <table className="m-0 w-max min-w-full border-collapse">
              <thead>
                <tr>
                  {sample.columns.map((c) => {
                    const kind = sampleColumnKind(c.name)
                    return (
                      <th
                        key={c.name}
                        className={cn(
                          "border-border-strong sticky top-0 z-2 border-b-2 bg-[color-mix(in_srgb,var(--table-header-bg)_92%,var(--bg-card))] p-[0.7rem_0.85rem] text-left align-top font-['Plus_Jakarta_Sans',sans-serif] text-[0.68rem] font-bold tracking-wider uppercase shadow-[0_1px_0_var(--table-header-shadow-line)] backdrop-blur-[6px]",
                          columnClass(kind),
                        )}
                      >
                        {c.name}
                      </th>
                    )
                  })}
                </tr>
              </thead>
              <tbody>
                {sample.rows.map((row, i) => (
                  <tr
                    key={i}
                    className={`group/row border-border border-b last:border-b-0 odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even) hover:bg-(--table-stripe-hover)`}
                  >
                    {row.map((cell, j) => {
                      const colName = sample.columns[j]?.name ?? ''
                      const kind = sampleColumnKind(colName)
                      return (
                        <td
                          key={j}
                          className={`p-[0.65rem_0.85rem] align-top ${columnClass(kind)}`}
                        >
                          <SampleCell value={cell} columnName={colName} />
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
      {!loading && sample && rowCount === 0 && (
        <p
          className={
            'border-border bg-card-raised text-foreground-muted mt-2 rounded-[0.55rem] border border-dashed p-[1.25rem_1rem] text-center text-[0.88rem]'
          }
        >
          {t('ai_query.sample_modal_empty')}
        </p>
      )}
    </Modal>
  )
}
