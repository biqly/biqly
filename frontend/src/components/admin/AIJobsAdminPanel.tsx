import { useCallback, useEffect, useMemo, useState } from 'react'

import { adminCancelAIJob, adminCancelAllStaleAIJobs, listAdminAIJobs } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { jobIsActive, jobQuestionPreview } from '../../hooks/useAIJobsUtils'
import { useConfirm } from '../../hooks/useConfirm'
import { useT } from '../../i18n'
import type { AIJob } from '../../types/ai'
import { formatDurationMs } from '../../utils/formatters'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'

const POLL_MS = 3000

const STATUS_OPTIONS = ['pending', 'queued', 'running', 'succeeded', 'failed', 'cancelled'] as const
const KIND_OPTIONS = ['run', 'preview', 'query', 'describe', 'describe_batch', 'embed_metadata']

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'succeeded':
      return 'success'
    case 'failed':
      return 'error'
    case 'cancelled':
      return 'clarification'
    default:
      return 'active'
  }
}

function jobElapsedMs(job: AIJob): number {
  const start = new Date(job.created_at).getTime()
  const end = job.finished_at ? new Date(job.finished_at).getTime() : Date.now()
  return Math.max(0, end - start)
}

export function AIJobsAdminPanel() {
  const t = useT()
  const confirm = useConfirm()
  const { accessToken } = useAuth()
  const { users } = useAdminLookups(accessToken ?? '')
  const [jobs, setJobs] = useState<AIJob[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [kindFilter, setKindFilter] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyJobId, setBusyJobId] = useState<string | null>(null)
  const [staleNote, setStaleNote] = useState<string | null>(null)

  const userLabelByID = useMemo(() => {
    const map = new Map<string, string>()
    for (const u of users) {
      const displayName = u.displayName?.trim() ?? ''
      const email = u.email.trim()
      map.set(u.id, displayName || email || u.id)
    }
    return map
  }, [users])

  const refresh = useCallback(async () => {
    if (!accessToken) {
      return
    }
    try {
      const res = await listAdminAIJobs(accessToken, {
        status: statusFilter || undefined,
        kind: kindFilter || undefined,
      })
      setJobs(res.jobs)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [accessToken, statusFilter, kindFilter])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void refresh()
    const id = window.setInterval(() => {
      void refresh()
    }, POLL_MS)
    return () => window.clearInterval(id)
  }, [refresh])

  const cancelJob = async (job: AIJob) => {
    if (!accessToken) {
      return
    }
    const ok = await confirm({
      title: t('admin.ai_jobs.cancel_confirm_title'),
      message: t('admin.ai_jobs.cancel_confirm_message', {
        request: jobQuestionPreview(job.kind, job.request_json),
      }),
      confirmLabel: t('admin.ai_jobs.cancel'),
    })
    if (!ok) {
      return
    }
    setBusyJobId(job.id)
    try {
      await adminCancelAIJob(accessToken, job.id)
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusyJobId(null)
    }
  }

  const cancelStale = async () => {
    if (!accessToken) {
      return
    }
    try {
      const res = await adminCancelAllStaleAIJobs(accessToken)
      setStaleNote(
        t('admin.ai_jobs.cancel_stale_done', {
          cancelled: res.cancelled,
          matched: res.matched,
        }),
      )
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const statusOptions = useMemo(
    () => [
      { value: '', label: t('admin.ai_jobs.filter_status_all') },
      ...STATUS_OPTIONS.map((s) => ({ value: s, label: t(`admin.ai_jobs.status_${s}`) })),
    ],
    [t],
  )
  const kindOptions = useMemo(
    () => [
      { value: '', label: t('admin.ai_jobs.filter_kind_all') },
      ...KIND_OPTIONS.map((k) => ({ value: k, label: k })),
    ],
    [t],
  )

  const thStyle: React.CSSProperties = {
    textAlign: 'left',
    padding: '0.6rem 0.75rem',
    fontSize: '0.75rem',
    fontWeight: 600,
    color: 'var(--text-muted)',
    borderBottom: '1px solid var(--border)',
  }
  const tdStyle: React.CSSProperties = {
    padding: '0.65rem 0.75rem',
    fontSize: '0.875rem',
    borderBottom: '1px solid var(--border-subtle, var(--border))',
    verticalAlign: 'top',
  }

  return (
    <div className="admin-panel">
      <div className="admin-panel__header">
        <div>
          <h2>{t('admin.ai_jobs.title')}</h2>
          <p style={{ color: 'var(--text-muted)', margin: 0 }}>{t('admin.ai_jobs.description')}</p>
        </div>
      </div>

      <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center', margin: '0.75rem 0' }}>
        <Select value={statusFilter} onChange={setStatusFilter} options={statusOptions} />
        <Select value={kindFilter} onChange={setKindFilter} options={kindOptions} />
        <button type="button" className="btn btn-sm" onClick={() => void cancelStale()}>
          {t('admin.ai_jobs.cancel_stale')}
        </button>
        {staleNote && <span style={{ color: 'var(--text-muted)' }}>{staleNote}</span>}
      </div>

      {error && <p className="error-text">{error}</p>}

      <div
        className="ai-history__table-wrap admin-table-container"
        style={{ position: 'relative' }}
      >
        <LoadingOverlay loading={loading} />
        <table className="ai-history__table" style={{ borderCollapse: 'collapse', width: '100%' }}>
          <thead>
            <tr>
              <th style={thStyle}>{t('admin.ai_jobs.col_user')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_kind')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_request')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_status')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_phase')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_progress')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_elapsed')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_created')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_actions')}</th>
            </tr>
          </thead>
          <tbody>
            {!loading && jobs.length === 0 ? (
              <tr>
                <td colSpan={9} style={{ ...tdStyle, color: 'var(--text-muted)' }}>
                  {t('admin.ai_jobs.empty')}
                </td>
              </tr>
            ) : (
              jobs.map((job) => {
                const active = jobIsActive(job)
                const userLabel = job.user_id
                  ? (userLabelByID.get(job.user_id) ?? job.user_id)
                  : '—'
                return (
                  <tr key={job.id}>
                    <td style={tdStyle}>{userLabel}</td>
                    <td style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)' }}>
                      {job.kind}
                    </td>
                    <td style={tdStyle}>{jobQuestionPreview(job.kind, job.request_json)}</td>
                    <td style={tdStyle}>
                      <span
                        className={`ai-history__status ai-history__status--${statusBadgeClass(job.status)}`}
                      >
                        {t(`admin.ai_jobs.status_${job.status}`)}
                      </span>
                    </td>
                    <td style={tdStyle}>
                      {job.phase}
                      {job.phase_message ? (
                        <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                          {job.phase_message}
                        </div>
                      ) : null}
                    </td>
                    <td style={tdStyle}>{active ? `${job.progress_pct}%` : '—'}</td>
                    <td style={tdStyle}>{formatDurationMs(jobElapsedMs(job))}</td>
                    <td style={tdStyle}>{new Date(job.created_at).toLocaleString()}</td>
                    <td style={tdStyle}>
                      {active && (
                        <button
                          type="button"
                          className="btn btn-sm btn-danger"
                          disabled={busyJobId === job.id}
                          onClick={() => void cancelJob(job)}
                        >
                          {busyJobId === job.id
                            ? t('admin.ai_jobs.cancelling')
                            : t('admin.ai_jobs.cancel')}
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
