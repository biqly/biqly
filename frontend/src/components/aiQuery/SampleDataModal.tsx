import { useEffect, useMemo, useState } from 'react'

import { useT } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'
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
      <span className="block text-foreground leading-[1.45] break-words text-foreground-faint opacity-55">
        —
      </span>
    )
  }

  if (kind === 'handle') {
    const handle = text.startsWith('@') ? text : `@${text}`
    return (
      <span className="inline-flex items-center max-w-full px-[0.45rem] py-[0.15rem] rounded-full border border-[color-mix(in_srgb,var(--accent)_22%,var(--border))] bg-[color-mix(in_srgb,var(--accent)_6%,transparent)] text-accent font-mono text-[0.78rem] font-semibold whitespace-nowrap overflow-hidden text-ellipsis">
        {handle}
      </span>
    )
  }

  if (kind === 'id') {
    return (
      <span className="font-mono text-[0.78rem] [font-variant-numeric:tabular-nums] text-foreground-muted break-all">
        {text}
      </span>
    )
  }

  if (kind === 'text') {
    return (
      <span
        className="line-clamp-4 overflow-hidden text-[0.84rem] text-foreground-muted group-hover/row:text-foreground"
        title={text}
      >
        {text}
      </span>
    )
  }

  return <span className="block text-foreground leading-[1.45] break-words">{text}</span>
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
        <span className="inline-flex items-baseline flex-wrap gap-0 mt-[0.35rem] font-mono text-[0.82rem] leading-[1.35]">
          <span className="text-accent font-semibold">{tableRef.schema}</span>
          <span className="text-foreground-muted opacity-70" aria-hidden="true">
            .
          </span>
          <span className="text-foreground-muted font-medium">{tableRef.table}</span>
        </span>
      }
      onClose={onClose}
      labelledBy="sample-data-title"
      className="!w-[min(96vw,72rem)] max-[720px]:!w-[min(100%,100vw-1rem)] !max-h-[min(88vh,52rem)] max-[720px]:!max-h-[92vh] flex flex-col"
      bodyClassName="!flex !flex-col gap-3 p-[0.85rem_1.1rem_1.1rem] max-[720px]:p-[0.75rem_0.85rem_0.9rem] min-h-0 flex-1"
    >
      <LoadingOverlay loading={loading} />
      {!loading && sample && rowCount > 0 && (
        <>
          <div className="flex items-center justify-between gap-3" aria-live="polite">
            <span
              className={legacyCardClass(
                'inline-flex items-center px-[0.65rem] py-[0.28rem] rounded-full border border-[color-mix(in_srgb,var(--accent)_28%,var(--border))] bg-[color-mix(in_srgb,var(--accent)_8%,var(--bg-card-raised))] text-foreground-muted text-[0.76rem] font-semibold tracking-wide',
              )}
            >
              {t('ai_query.sample_modal_meta', { rows: rowCount, cols: colCount })}
            </span>
          </div>
          <div
            className={legacyCardClass(
              'flex-1 min-h-0 max-h-[min(62vh,40rem)] max-[720px]:max-h-[58vh] overflow-auto border border-border rounded-[0.65rem] bg-card-raised shadow-[inset_0_1px_0_color-mix(in_srgb,var(--text-primary)_4%,transparent)] overscroll-contain custom-scrollbar-thin',
            )}
          >
            <table className="m-0 min-w-full w-max border-collapse">
              <thead>
                <tr>
                  {sample.columns.map((c) => {
                    const kind = sampleColumnKind(c.name)
                    return (
                      <th
                        key={c.name}
                        className={cn(
                          legacyCardClass(
                            "sticky top-0 z-[2] p-[0.7rem_0.85rem] text-[0.68rem] text-left align-top bg-[color-mix(in_srgb,var(--table-header-bg)_92%,var(--bg-card))] backdrop-blur-[6px] font-['Plus_Jakarta_Sans',sans-serif] font-bold tracking-wider uppercase border-b-2 border-[var(--border-strong)] shadow-[0_1px_0_var(--table-header-shadow-line)]",
                          ),
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
                    className={`group/row border-b border-border last:border-b-0 odd:bg-[var(--table-stripe-odd)] even:bg-[var(--table-stripe-even)] hover:bg-[var(--table-stripe-hover)]`}
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
          className={legacyCardClass(
            'mt-2 p-[1.25rem_1rem] border border-dashed border-border rounded-[0.55rem] bg-card-raised text-foreground-muted text-[0.88rem] text-center',
          )}
        >
          {t('ai_query.sample_modal_empty')}
        </p>
      )}
    </Modal>
  )
}
