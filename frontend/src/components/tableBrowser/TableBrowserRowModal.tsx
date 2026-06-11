import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import type { useT } from '../../i18n'
import type { SemanticJoin } from '../../types/semantic'
import { formatResultCell } from '../../utils/resultCellFormat'
import { tableKey } from '../modeling/utils'
import { Modal } from '../ui/Modal'
import { rowTitleFor } from './rowTitle'
import { TableBrowserCellValue } from './TableBrowserCellValue'
import { buildTableRowsUrl, type TableRowsResult } from './useTableBrowserQueryState'

const RELATED_PAGE_SIZE = 25

type Translate = ReturnType<typeof useT>
type PostData = <T>(url: string, body: unknown) => Promise<T | null>

interface RowFrame {
  kind: 'row'
  schema: string
  table: string
  columns: string[]
  row: unknown[]
}

interface ListFrame {
  kind: 'list'
  schema: string
  table: string
  filterColumn: string
  filterValue: string
}

export type RowModalFrame = RowFrame | ListFrame

/** A join edge seen from the current table's perspective. */
interface RelatedLink {
  key: string
  schema: string
  table: string
  localColumn: string
  remoteColumn: string
  /** true when the related side holds at most one row for this value. */
  toOne: boolean
}

function relatedLinksFor(
  joins: SemanticJoin[],
  baseSchema: string,
  schema: string,
  table: string,
): RelatedLink[] {
  const out: RelatedLink[] = []
  for (const j of joins) {
    if (j.is_active === false) {
      continue
    }
    const fromSchema = j.from_schema ?? baseSchema
    const toSchema = j.to_schema ?? baseSchema
    if (fromSchema === schema && j.from_table === table) {
      out.push({
        key: `${j.id}-to`,
        schema: toSchema,
        table: j.to_table,
        localColumn: j.from_column,
        remoteColumn: j.to_column,
        toOne: j.relationship === 'many_to_one' || j.relationship === 'one_to_one',
      })
    }
    if (toSchema === schema && j.to_table === table) {
      out.push({
        key: `${j.id}-from`,
        schema: fromSchema,
        table: j.from_table,
        localColumn: j.to_column,
        remoteColumn: j.from_column,
        toOne: j.relationship === 'one_to_many' || j.relationship === 'one_to_one',
      })
    }
  }
  return out
}

function eqFilterBody(column: string, value: string, limit: number, offset = 0) {
  return {
    filters: [{ column, operator: 'eq', value, case_sensitive: true }],
    limit,
    offset,
    include_total: true,
  }
}

function cellString(v: unknown): string | null {
  if (v == null) {
    return null
  }
  if (typeof v === 'string') {
    return v
  }
  if (typeof v === 'number' || typeof v === 'boolean') {
    return String(v)
  }
  return null
}

/** One related-table card: count + inline value (to-one) or drill action. */
function RelatedLinkCard({
  link,
  value,
  datasourceId,
  displayExpressionByTable,
  postData,
  t,
  formatInt,
  onOpenRow,
  onOpenList,
}: {
  link: RelatedLink
  value: string | null
  datasourceId: string
  displayExpressionByTable: Map<string, string>
  postData: PostData
  t: Translate
  formatInt: (n: number) => string
  onOpenRow: (frame: RowFrame) => void
  onOpenList: (frame: ListFrame) => void
}) {
  const [state, setState] = useState<{
    loading: boolean
    total: number | null
    columns: string[]
    firstRow: unknown[] | null
  }>({ loading: true, total: null, columns: [], firstRow: null })

  useEffect(() => {
    if (value == null) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setState({ loading: false, total: null, columns: [], firstRow: null })
      return
    }
    let cancelled = false
    setState((prev) => ({ ...prev, loading: true }))
    void postData<TableRowsResult>(
      buildTableRowsUrl(datasourceId, link.schema, link.table),
      eqFilterBody(link.remoteColumn, value, 1),
    ).then((res) => {
      if (cancelled) {
        return
      }
      setState({
        loading: false,
        total: res?.total ?? null,
        columns: res?.columns?.map((c) => c.name) ?? [],
        firstRow: res?.rows?.[0] ?? null,
      })
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId, link.schema, link.table, link.remoteColumn, value, postData])

  const relatedKey = tableKey(link.schema, link.table)
  const title =
    state.firstRow != null
      ? rowTitleFor(
          state.firstRow,
          state.columns,
          displayExpressionByTable.get(relatedKey),
          relatedKey,
          relatedKey,
        )
      : null

  return (
    <div className="row-modal-related-card">
      <div className="row-modal-related-card__head">
        <span className="row-modal-related-card__table">{relatedKey}</span>
        <span className="row-modal-related-card__badge">{link.toOne ? '1:1 / N:1' : '1:N'}</span>
      </div>
      <code className="row-modal-related-card__on">
        {link.localColumn} = {link.remoteColumn}
      </code>
      {value == null ? (
        <p className="row-modal-related-card__empty">{t('table_browser.related_no_value')}</p>
      ) : state.loading ? (
        <p className="row-modal-related-card__empty">{t('table_browser.loading')}</p>
      ) : link.toOne ? (
        state.firstRow ? (
          <button
            type="button"
            className="row-modal-related-card__value"
            onClick={() =>
              state.firstRow &&
              onOpenRow({
                kind: 'row',
                schema: link.schema,
                table: link.table,
                columns: state.columns,
                row: state.firstRow,
              })
            }
          >
            {title} →
          </button>
        ) : (
          <p className="row-modal-related-card__empty">{t('table_browser.related_no_match')}</p>
        )
      ) : (
        <button
          type="button"
          className="row-modal-related-card__value"
          disabled={!state.total}
          onClick={() =>
            onOpenList({
              kind: 'list',
              schema: link.schema,
              table: link.table,
              filterColumn: link.remoteColumn,
              filterValue: value,
            })
          }
        >
          {t('table_browser.related_count', { count: formatInt(state.total ?? 0) })} →
        </button>
      )}
    </div>
  )
}

/** Related rows list for 1:N / N:N drill-through, loaded as the modal scroll reaches the end. */
function RelatedListView({
  frame,
  datasourceId,
  postData,
  t,
  formatInt,
  onOpenRow,
}: {
  frame: ListFrame
  datasourceId: string
  postData: PostData
  t: Translate
  formatInt: (n: number) => string
  onOpenRow: (frame: RowFrame) => void
}) {
  const [columns, setColumns] = useState<string[]>([])
  const [rows, setRows] = useState<unknown[][]>([])
  const [total, setTotal] = useState<number | null>(null)
  const [exhausted, setExhausted] = useState(false)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const loadMoreRef = useRef<HTMLDivElement | null>(null)
  const requestIdRef = useRef(0)

  const fetchPage = useCallback(
    async (offset: number, requestId: number) => {
      const res = await postData<TableRowsResult>(
        buildTableRowsUrl(datasourceId, frame.schema, frame.table),
        eqFilterBody(frame.filterColumn, frame.filterValue, RELATED_PAGE_SIZE, offset),
      )
      if (requestIdRef.current !== requestId) {
        return
      }
      const nextRows = res?.rows ?? []
      setColumns(res?.columns?.map((c) => c.name) ?? [])
      setTotal(res?.total ?? null)
      setExhausted(nextRows.length < RELATED_PAGE_SIZE)
      setRows((prev) => (offset === 0 ? nextRows : [...prev, ...nextRows]))
    },
    [datasourceId, frame.schema, frame.table, frame.filterColumn, frame.filterValue, postData],
  )

  useEffect(() => {
    const requestId = requestIdRef.current + 1
    requestIdRef.current = requestId
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRows([])
    setColumns([])
    setTotal(null)
    setExhausted(false)
    setLoading(true)
    void fetchPage(0, requestId).finally(() => {
      if (requestIdRef.current === requestId) {
        setLoading(false)
      }
    })
    return () => {
      requestIdRef.current += 1
    }
  }, [fetchPage])

  const hasMore = !exhausted && (total == null ? rows.length > 0 : rows.length < total)

  const loadMore = useCallback(() => {
    if (loading || loadingMore || !hasMore) {
      return
    }
    const requestId = requestIdRef.current
    setLoadingMore(true)
    void fetchPage(rows.length, requestId).finally(() => {
      if (requestIdRef.current === requestId) {
        setLoadingMore(false)
      }
    })
  }, [fetchPage, hasMore, loading, loadingMore, rows.length])

  useEffect(() => {
    const target = loadMoreRef.current
    if (!target || !hasMore) {
      return
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        loadMore()
      }
    })
    observer.observe(target)
    return () => {
      observer.disconnect()
    }
  }, [hasMore, loadMore])

  if (loading && rows.length === 0) {
    return <p className="row-modal-related-card__empty">{t('table_browser.loading')}</p>
  }
  return (
    <div className="row-modal-list">
      <div className="row-modal-list__scroll">
        <table className="results-table table-browser-grid">
          <thead>
            <tr>
              {columns.map((c) => (
                <th key={c} scope="col">
                  {c}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr
                key={i}
                className="table-browser-data-row"
                onClick={() =>
                  onOpenRow({
                    kind: 'row',
                    schema: frame.schema,
                    table: frame.table,
                    columns,
                    row,
                  })
                }
              >
                {columns.map((c, j) => {
                  const display = formatResultCell(row[j], c, {})
                  return (
                    <td key={c}>
                      <TableBrowserCellValue value={display} />
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="row-modal-list__footer">
        <span className="table-browser-range">
          {total != null
            ? t('table_browser.range_of_total', {
                start: formatInt(rows.length > 0 ? 1 : 0),
                end: formatInt(rows.length),
                total: formatInt(total),
              })
            : ''}
        </span>
        {loadingMore && <span className="table-browser-range">{t('table_browser.loading')}</span>}
      </div>
      <div ref={loadMoreRef} className="row-modal-list__sentinel" aria-hidden="true" />
    </div>
  )
}

/**
 * Row detail explorer: shows the clicked row's own fields plus its related
 * tables; to-one links open inline, to-many links drill into a filtered list.
 * A frame stack with a back button lets the user walk across tables.
 */
export function TableBrowserRowModal({
  open,
  onClose,
  datasourceId,
  joins,
  baseSchema,
  displayExpressionByTable,
  initialFrame,
  fallbackTitle,
  postData,
  t,
  formatInt,
}: {
  open: boolean
  onClose: () => void
  datasourceId: string
  joins: SemanticJoin[]
  baseSchema: string
  displayExpressionByTable: Map<string, string>
  initialFrame: RowFrame | null
  fallbackTitle: string
  postData: PostData
  t: Translate
  formatInt: (n: number) => string
}) {
  const [stack, setStack] = useState<RowModalFrame[]>([])
  const frames = stack.length > 0 ? stack : initialFrame ? [initialFrame] : []
  const frame = frames[frames.length - 1] ?? null

  // Reset the navigation stack whenever a new row is opened from the grid.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setStack(initialFrame ? [initialFrame] : [])
  }, [initialFrame])

  const pushFrame = useCallback((next: RowModalFrame) => {
    setStack((prev) => [...prev, next])
  }, [])

  const popFrame = useCallback(() => {
    setStack((prev) => (prev.length > 1 ? prev.slice(0, -1) : prev))
  }, [])

  const links = useMemo(() => {
    if (frame?.kind !== 'row') {
      return []
    }
    return relatedLinksFor(joins, baseSchema, frame.schema, frame.table)
  }, [joins, baseSchema, frame])

  if (!frame) {
    return null
  }

  const frameTableKey = tableKey(frame.schema, frame.table)
  const title =
    frame.kind === 'row'
      ? rowTitleFor(
          frame.row,
          frame.columns,
          displayExpressionByTable.get(frameTableKey),
          fallbackTitle,
          frameTableKey,
        )
      : `${frameTableKey} · ${frame.filterColumn} = ${frame.filterValue}`

  const colIndex =
    frame.kind === 'row' ? new Map(frame.columns.map((c, i) => [c, i])) : new Map<string, number>()

  return (
    <Modal
      open={open}
      title={title}
      subtitle={frameTableKey}
      onClose={onClose}
      className="modal-card--xl"
      bodyClassName="table-browser-detail-modal-body"
    >
      {frames.length > 1 && (
        <div className="row-modal-nav">
          <button type="button" className="row-modal-back" onClick={popFrame}>
            ‹ {t('table_browser.back')}
          </button>
          <span className="row-modal-nav__path">
            {frames.map((f, i) => (
              <span key={i} className="row-modal-nav__crumb">
                {i > 0 && ' › '}
                {f.table}
              </span>
            ))}
          </span>
        </div>
      )}

      {frame.kind === 'row' ? (
        <div className="row-modal-layout">
          <div
            className="table-browser-detail-grid"
            role="region"
            aria-label={t('table_browser.row_detail')}
          >
            {frame.columns.map((colName) => {
              const j = colIndex.get(colName)
              const display = formatResultCell(j != null ? frame.row[j] : null, colName, {})
              return (
                <div key={colName} className="table-browser-detail-item">
                  <span className="table-browser-detail-label">{colName}</span>
                  <TableBrowserCellValue
                    value={display}
                    className="table-browser-detail-value"
                    multiline
                  />
                </div>
              )
            })}
          </div>
          {links.length > 0 && (
            <aside className="row-modal-related" aria-label={t('table_browser.related_tables')}>
              <h4 className="row-modal-related__title">{t('table_browser.related_tables')}</h4>
              {links.map((link) => {
                const j = colIndex.get(link.localColumn)
                return (
                  <RelatedLinkCard
                    key={link.key}
                    link={link}
                    value={cellString(j != null ? frame.row[j] : null)}
                    datasourceId={datasourceId}
                    displayExpressionByTable={displayExpressionByTable}
                    postData={postData}
                    t={t}
                    formatInt={formatInt}
                    onOpenRow={pushFrame}
                    onOpenList={pushFrame}
                  />
                )
              })}
            </aside>
          )}
        </div>
      ) : (
        <RelatedListView
          frame={frame}
          datasourceId={datasourceId}
          postData={postData}
          t={t}
          formatInt={formatInt}
          onOpenRow={pushFrame}
        />
      )}
    </Modal>
  )
}
