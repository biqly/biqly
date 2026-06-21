import { useCallback, useEffect, useMemo, useState } from 'react'

import { adminCancelAIJob, adminCancelAllStaleAIJobs, listAdminAIJobs } from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { jobIsActive, jobQuestionPreview } from '../../hooks/useAIJobsUtils'
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import type { AIJob } from '../../types/ai'
import type { PageQuery } from '../../types/pagination'
import { errorMessage } from '../../utils/error'
import { formatDateTime, formatDurationMs } from '../../utils/formatters'
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
import { adminTableContainerClass, jobDetailModalClass } from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
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
  const [locale] = useLocale()
  const runConfirmedMutation = useConfirmedMutation()
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
    setBusyJobId(job.id)
    const ok = await runConfirmedMutation(() => adminCancelAIJob(accessToken, job.id), {
      title: t('admin.ai_jobs.cancel_confirm_title'),
      message: t('admin.ai_jobs.cancel_confirm_message', {
        request: jobQuestionPreview(job.kind, job.request_json),
      }),
    })
    setBusyJobId(null)
    if (ok) {
      reload()
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
      setError(errorMessage(err))
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

  const thClass =
    'text-left px-3 py-2.5 text-xs font-semibold text-foreground-muted border-b border-border'
  const tdClass = 'px-3 py-2.5 text-sm border-b border-border/40 align-top'

  return (
    <AdminPanelShell
      title={t('admin.ai_jobs.title')}
      description={t('admin.ai_jobs.description')}
      error={error}
    >
      <div className="mb-2 flex items-center gap-3">
        <Select value={statusFilter} onChange={setStatusFilter} options={statusOptions} />
        <Select value={kindFilter} onChange={setKindFilter} options={kindOptions} />
        <button
          type="button"
          className={buttonClass('secondary', { size: 'sm' })}
          onClick={() => void cancelStale()}
        >
          {t('admin.ai_jobs.cancel_stale')}
        </button>
        {staleNote && <span className="text-foreground-muted">{staleNote}</span>}
      </div>

      <div className={cn(aiHistoryTableWrapClass, adminTableContainerClass, 'relative')}>
        <LoadingOverlay loading={loading && jobs.length === 0} />
        <table className={cn(aiHistoryTableClass, 'w-full border-collapse')}>
          <thead>
            <tr>
              <th className={thClass}>{t('admin.ai_jobs.col_user')}</th>
              <th className={thClass}>{t('admin.ai_jobs.col_kind')}</th>
              <th className={thClass}>{t('admin.ai_jobs.col_request')}</th>
              <th className={thClass}>{t('admin.ai_jobs.col_status')}</th>
              <th className={thClass}>{t('admin.ai_jobs.col_created')}</th>
            </tr>
          </thead>
          <tbody>
            {!loading && jobs.length === 0 ? (
              <tr>
                <td colSpan={5} className={cn(tdClass, 'text-foreground-muted')}>
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
                    <td className={tdClass}>{userLabel}</td>
                    <td className={cn(tdClass, 'font-mono')}>{job.kind}</td>
                    <td className={tdClass}>{jobQuestionPreview(job.kind, job.request_json)}</td>
                    <td className={tdClass}>
                      <span className={aiHistoryStatusClass(statusBadgeClass(job.status))}>
                        {t(`admin.ai_jobs.status_${job.status}`)}
                      </span>
                    </td>
                    <td className={tdClass}>
                      {formatDateTime(job.created_at, localeLanguageTag(locale))}
                    </td>
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
                <div className={cn(jobDetailValueClass, 'mt-1')}>
                  <span className={aiHistoryStatusClass(statusBadgeClass(selectedJob.status))}>
                    {t(`admin.ai_jobs.status_${selectedJob.status}`)}
                  </span>
                </div>
              </div>

              <div className={jobDetailItemClass}>
                <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_phase')}</span>
                <span className={jobDetailValueClass}>{selectedJob.phase || '—'}</span>
                {selectedJob.phase_message && (
                  <div className="text-foreground-muted text-caption mt-0.5">
                    {selectedJob.phase_message}
                  </div>
                )}
              </div>

              {jobIsActive(selectedJob) && (
                <div className={jobDetailItemClass}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_progress')}</span>
                  <div className={cn(jobDetailValueClass, 'flex items-center gap-2')}>
                    <div className={cn(jobProgressBarClass, 'flex-1')}>
                      <div
                        className={jobProgressBarFillClass}
                        style={{ width: `${selectedJob.progress_pct}%` }}
                      />
                    </div>
                    <span className="text-caption font-semibold">{selectedJob.progress_pct}%</span>
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
                  {formatDateTime(selectedJob.created_at, localeLanguageTag(locale))}
                </span>
              </div>

              {selectedJob.finished_at && (
                <div className={jobDetailItemClass}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_finished')}</span>
                  <span className={jobDetailValueClass}>
                    {formatDateTime(selectedJob.finished_at, localeLanguageTag(locale))}
                  </span>
                </div>
              )}

              {jobIsActive(selectedJob) && (
                <div className={cn(jobDetailItemClass, 'mt-2')}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.col_actions')}</span>
                  <div className="mt-1">
                    <button
                      type="button"
                      className={cn(buttonClass('danger'), 'w-full')}
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

            <div className="flex min-w-0 flex-col gap-4">
              {selectedJob.error_message && (
                <div className={jobDetailItemClass}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.payload_error')}</span>
                  <div className={jobErrorBlockClass}>{selectedJob.error_message}</div>
                </div>
              )}

              {selectedJob.request_json != null && (
                <div className={cn(jobDetailItemClass, 'min-w-0')}>
                  <span className={jobDetailLabelClass}>{t('admin.ai_jobs.payload_request')}</span>
                  <pre className={jobJsonBlockClass}>
                    {JSON.stringify(selectedJob.request_json, null, 2)}
                  </pre>
                </div>
              )}

              {selectedJob.result_json != null && (
                <div className={cn(jobDetailItemClass, 'min-w-0')}>
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
    </AdminPanelShell>
  )
}
