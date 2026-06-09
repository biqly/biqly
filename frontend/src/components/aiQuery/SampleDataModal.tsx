import '../../styles/sample-data-modal.css'

import { useEffect, useMemo, useState } from 'react'

import { useT } from '../../i18n'
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
    return <span className="sample-cell sample-cell--empty">—</span>
  }

  if (kind === 'handle') {
    const handle = text.startsWith('@') ? text : `@${text}`
    return <span className="sample-cell sample-cell--handle">{handle}</span>
  }

  if (kind === 'id') {
    return <span className="sample-cell sample-cell--id">{text}</span>
  }

  if (kind === 'text') {
    return (
      <span className="sample-cell sample-cell--text" title={text}>
        {text}
      </span>
    )
  }

  return <span className="sample-cell">{text}</span>
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
        <span className="sample-modal-subtitle">
          <span className="sample-modal-schema">{tableRef.schema}</span>
          <span className="sample-modal-sep" aria-hidden="true">
            .
          </span>
          <span className="sample-modal-table">{tableRef.table}</span>
        </span>
      }
      onClose={onClose}
      labelledBy="sample-data-title"
      className="modal-card--sample-data"
      bodyClassName="modal-body--sample-data"
    >
      <LoadingOverlay loading={loading} />
      {!loading && sample && rowCount > 0 && (
        <>
          <div className="sample-modal-meta" aria-live="polite">
            <span className="sample-modal-meta-pill">
              {t('ai_query.sample_modal_meta', { rows: rowCount, cols: colCount })}
            </span>
          </div>
          <div className="sample-modal-table-wrap">
            <table className="results-table results-table--sample-preview">
              <thead>
                <tr>
                  {sample.columns.map((c) => {
                    const kind = sampleColumnKind(c.name)
                    return (
                      <th key={c.name} className={`sample-col sample-col--${kind}`}>
                        {c.name}
                      </th>
                    )
                  })}
                </tr>
              </thead>
              <tbody>
                {sample.rows.map((row, i) => (
                  <tr key={i}>
                    {row.map((cell, j) => {
                      const colName = sample.columns[j]?.name ?? ''
                      const kind = sampleColumnKind(colName)
                      return (
                        <td key={j} className={`sample-col sample-col--${kind}`}>
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
        <p className="sample-modal-empty">{t('ai_query.sample_modal_empty')}</p>
      )}
    </Modal>
  )
}
