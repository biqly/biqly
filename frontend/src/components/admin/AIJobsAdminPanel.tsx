import { useCallback, useEffect, useMemo, useState } from 'react'

import { adminCancelAIJob, adminCancelAllStaleAIJobs, listAdminAIJobs } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { jobIsActive, jobQuestionPreview } from '../../hooks/useAIJobsUtils'
import { useConfirm } from '../../hooks/useConfirm'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import type { AIJob } from '../../types/ai'
import type { PageQuery } from '../../types/pagination'
import { formatDurationMs } from '../../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../utils/paging'
import {
  aiHistoryStatusClass,
  type AiHistoryStatusVariant,
  aiHistoryTableClass,
  aiHistoryTableWrapClass,
  aiJobsTableRowClass,
  jobDetailGridClass,
  jobDetailItemClass,
  jobDetailLabelClass,
  jobDetailSectionClass,
  jobDetailValueClass,
  jobDetailValueMonoClass,
  jobErrorBlockClass,
  jobJsonBlockClass,
  jobProgressBarClass,
  jobProgressBarFillClass,
} from '../ai/aiJobsClasses'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Modal } from '../ui/Modal'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  adminPanelClass,
  adminPanelHeaderClass,
  adminTableContainerClass,
  jobDetailModalClass,
} from './adminClasses'
const POLL_MS = 3000

const DEFAULT_AI_JOBS_PAGE_SIZE = 25

const STATUS_OPTIONS = ['pending', 'queued', 'running', 'succeeded', 'failed', 'cancelled'] as const
const KIND_OPTIONS = ['run', 'preview', 'query', 'describe', 'describe_batch', 'embed_metadata']

function statusBadgeClass(status: string): AiHistoryStatusVariant {
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
  const [statusFilter, setStatusFilter] = useState('')
  const [kindFilter, setKindFilter] = useState('')
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

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listAdminAIJobs(accessToken ?? '', {
        status: statusFilter || undefined,
        kind: kindFilter || undefined,
        page: q.page,
        pageSize: q.pageSize,
      })
      return { items: res.jobs, total: res.total }
    },
    [accessToken, statusFilter, kindFilter],
  )

  const {
    items: jobs,
    loading,
    error,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    setPageSize,
    totalPages,
    total: totalItems,
    reload,
    setError,
  } = usePaginatedList<AIJob>({
    fetcher,
    initialPageSize: DEFAULT_AI_JOBS_PAGE_SIZE,
    enabled: Boolean(accessToken),
    fetchKey: accessToken,
    resetPageKey: `${statusFilter}|${kindFilter}`,
    syncToUrl: 'aiJobsPage',
  })

  const [selectedJobId, setSelectedJobId] = useState<string | null>(null)
  const [lastClickedJob, setLastClickedJob] = useState<AIJob | null>(null)

  const selectedJob = useMemo(() => {
    if (!selectedJobId) {
      return null
    }
    return jobs.find((j) => j.id === selectedJobId) ?? lastClickedJob
  }, [selectedJobId, jobs, lastClickedJob])

  useEffect(() => {
    if (!accessToken) {
      return
    }
    const id = window.setInterval(() => reload(), POLL_MS)
    return () => window.clearInterval(id)
  }, [accessToken, reload])

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
      reload()
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
      reload()
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
    <div className={adminPanelClass}>
      <div className={adminPanelHeaderClass}>
        <div>
          <h2>{t('admin.ai_jobs.title')}</h2>
          <p style={{ color: 'var(--text-muted)', margin: 0 }}>{t('admin.ai_jobs.description')}</p>
        </div>
      </div>

      <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center', margin: '0.75rem 0' }}>
        <Select value={statusFilter} onChange={setStatusFilter} options={statusOptions} />
        <Select value={kindFilter} onChange={setKindFilter} options={kindOptions} />
        <button
          type="button"
          className={legacyButtonClass('btn btn-sm')}
          onClick={() => void cancelStale()}
        >
          {t('admin.ai_jobs.cancel_stale')}
        </button>
        {staleNote && <span style={{ color: 'var(--text-muted)' }}>{staleNote}</span>}
      </div>

      {error && <p className={legacyFeedbackClass('error-text')}>{error}</p>}

      <div
        className={`${aiHistoryTableWrapClass} ${adminTableContainerClass}`}
        style={{ position: 'relative' }}
      >
        <LoadingOverlay loading={loading && jobs.length === 0} />
        <table
          className={aiHistoryTableClass}
          style={{ borderCollapse: 'collapse', width: '100%' }}
        >
          <thead>
            <tr>
              <th style={thStyle}>{t('admin.ai_jobs.col_user')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_kind')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_request')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_status')}</th>
              <th style={thStyle}>{t('admin.ai_jobs.col_created')}</th>
            </tr>
          </thead>
          <tbody>
            {!loading && jobs.length === 0 ? (
              <tr>
                <td colSpan={5} style={{ ...tdStyle, color: 'var(--text-muted)' }}>
                  {t('admin.ai_jobs.empty')}
                </td>
              </tr>
            ) : (
              jobs.map((job) => {
                const userLabel = job.user_id
                  ? (userLabelByID.get(job.user_id) ?? job.user_id)
                  : '—'
                return (
                  <tr
                    key={job.id}
                    className={aiJobsTableRowClass}
                    title={t('admin.ai_jobs.click_to_view')}
                    onClick={() => {
                      setSelectedJobId(job.id)
                      setLastClickedJob(job)
                    }}
                  >
                    <td style={tdStyle}>{userLabel}</td>
                    <td style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)' }}>
                      {job.kind}
                    </td>
                    <td style={tdStyle}>{jobQuestionPreview(job.kind, job.request_json)}</td>
                    <td style={tdStyle}>
                      <span className={aiHistoryStatusClass(statusBadgeClass(job.status))}>
                        {t(`admin.ai_jobs.status_${job.status}`)}
                      </span>
                    </td>
                    <td style={tdStyle}>{new Date(job.created_at).toLocaleString()}</td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>

        {jobs.length > 0 && (
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={setCurrentPage}
            totalItems={totalItems}
            itemsPerPage={pageSize}
            pageSizeOptions={DEFAULT_TABLE_PAGE_SIZE_OPTIONS}
            onPageSizeChange={(size) => {
              setPageSize(size)
              setCurrentPage(1)
            }}
          />
        )}
      </div>

      {selectedJob && (
        <Modal
          open={true}
          title={t('admin.ai_jobs.detail_title')}
          onClose={() => {
            setSelectedJobId(null)
            setLastClickedJob(null)
          }}
          className={jobDetailModalClass}
          bodyClassName="job-detail-modal-body"
        >
          <div className={jobDetailGridClass}>
            <div className={jobDetailSectionClass}>
              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_id')}</span>
                <span className={`${jobDetailValueClass} ${jobDetailValueMonoClass}`}>
                  {selectedJob.id}
                </span>
              </div>

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_user')}</span>
                <span className={jobDetailValueClass}>
                  {selectedJob.user_id
                    ? (userLabelByID.get(selectedJob.user_id) ?? selectedJob.user_id)
                    : '—'}
                </span>
              </div>

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_kind')}</span>
                <span className={`${jobDetailValueClass} ${jobDetailValueMonoClass}`}>
                  {selectedJob.kind}
                </span>
              </div>

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_status')}</span>
                <div className={jobDetailValueClass} style={{ marginTop: '0.2rem' }}>
                  <span className={aiHistoryStatusClass(statusBadgeClass(selectedJob.status))}>
                    {t(`admin.ai_jobs.status_${selectedJob.status}`)}
                  </span>
                </div>
              </div>

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_phase')}</span>
                <span className={jobDetailValueClass}>{selectedJob.phase || '—'}</span>
                {selectedJob.phase_message && (
                  <div
                    style={{
                      color: 'var(--text-muted)',
                      fontSize: '0.75rem',
                      marginTop: '0.15rem',
                    }}
                  >
                    {selectedJob.phase_message}
                  </div>
                )}
              </div>

              {jobIsActive(selectedJob) && (
                <div className={jobDetailItemClass}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_progress')}</span>
                  <div
                    className={jobDetailValueClass}
                    style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
                  >
                    <div className={jobProgressBarClass} style={{ flex: 1 }}>
                      <div
                        className={jobProgressBarFillClass}
                        style={{ width: `${selectedJob.progress_pct}%` }}
                      />
                    </div>
                    <span style={{ fontSize: '0.75rem', fontWeight: 600 }}>
                      {selectedJob.progress_pct}%
                    </span>
                  </div>
                </div>
              )}

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.active_duration')}</span>
                <span className={jobDetailValueClass}>
                  {formatDurationMs(jobElapsedMs(selectedJob))}
                </span>
              </div>

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_created')}</span>
                <span className={jobDetailValueClass}>
                  {new Date(selectedJob.created_at).toLocaleString()}
                </span>
              </div>

              {selectedJob.finished_at && (
                <div className={jobDetailItemClass}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_finished')}</span>
                  <span className={jobDetailValueClass}>
                    {new Date(selectedJob.finished_at).toLocaleString()}
                  </span>
                </div>
              )}

              {jobIsActive(selectedJob) && (
                <div className={jobDetailItemClass} style={{ marginTop: '0.5rem' }}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_actions')}</span>
                  <div style={{ marginTop: '0.25rem' }}>
                    <button
                      type="button"
                      className={legacyButtonClass('btn btn-danger')}
                      style={{ width: '100%' }}
                      disabled={busyJobId === selectedJob.id}
                      onClick={() => void cancelJob(selectedJob)}
                    >
                      {busyJobId === selectedJob.id
                        ? t('admin.ai_jobs.cancelling')
                        : t('admin.ai_jobs.cancel')}
                    </button>
                  </div>
                </div>
              )}
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', minWidth: 0 }}>
              {selectedJob.error_message && (
                <div className={jobDetailItemClass}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.payload_error')}</span>
                  <div className={jobErrorBlockClass}>{selectedJob.error_message}</div>
                </div>
              )}

              {selectedJob.request_json != null && (
                <div className={jobDetailItemClass} style={{ minWidth: 0 }}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.payload_request')}</span>
                  <pre className={jobJsonBlockClass}>
                    {JSON.stringify(selectedJob.request_json, null, 2)}
                  </pre>
                </div>
              )}

              {selectedJob.result_json != null && (
                <div className={jobDetailItemClass} style={{ minWidth: 0 }}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.payload_result')}</span>
                  <pre className={jobJsonBlockClass}>
                    {JSON.stringify(selectedJob.result_json, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
