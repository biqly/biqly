import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import type { AIJob, AIJobKind, AIJobListResponse, AIQueryResponse, DescribeBatchJobProgress } from '../types/ai'
import type { DescribeBatchConflictBody } from '../api/describeBatchConflict'
import { runMetadataDescribeDirect } from '../api/metadataDescribe'
import type { DescribeBatchResult, DescribeResult } from '../types/metadata'
import type { BulkEntry } from '../components/metadata/bulkProgress'
import { getLocale } from '../i18n'
import { getAIClientSessionId } from '../utils/aiSession'
import { createJobWaiter, type JobCallbacks, type JobWaiterHandle } from './jobWaiter'
import { fetchJSON, type FetchJSONResult } from '../api/apiClient'
export { fetchJSON }
export type { FetchJSONResult }

const POLL_MS = 1200
const TERMINAL = new Set(['succeeded', 'failed', 'cancelled'])

function isValidJobId(id: unknown): id is string {
  if (typeof id !== 'string') return false
  const trimmed = id.trim()
  return trimmed.length > 0 && trimmed !== 'undefined' && trimmed !== 'null'
}

function jobEnqueueError(data: unknown, status: number): string {
  if (data && typeof data === 'object') {
    const err = (data as Record<string, unknown>).error
    if (typeof err === 'string' && err.trim()) return err.trim()
  }
  return `Failed to enqueue AI job (${status})`
}

export function jobIsActive(job: AIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

export type TrackedAIJob = AIJob & {
  questionPreview?: string
}

export type { JobCallbacks } from './jobWaiter'

export type BulkDescribeSummary = { ok: number; error: number; skipped: number }

export type BulkDescribeTarget = {
  schema_name: string
  table_name: string
  description: string | null
}

type AIJobsContextValue = {
  sessionId: string
  jobs: TrackedAIJob[]
  expanded: boolean
  setExpanded: (v: boolean) => void
  minimized: boolean
  setMinimized: (v: boolean) => void
  dismissJob: (id: string) => void
  cancelJob: (id: string) => Promise<boolean>
  cancelAllActiveJobs: () => Promise<number>
  listStaleJobs: (olderMinutes?: number) => Promise<AIJob[]>
  cancelJobIds: (ids: string[]) => Promise<number>
  runJob: <TRequest extends object, TResult = AIQueryResponse>(
    kind: AIJobKind,
    request: TRequest,
    callbacks?: JobCallbacks<TResult>,
  ) => Promise<TResult | 'fallback' | null>
  bulkDescribe: {
    running: boolean
    entries: BulkEntry[]
    summary: BulkDescribeSummary | null
    start: (opts: {
      datasourceId: string
      targets: BulkDescribeTarget[]
      sampleSize: number
      skipExisting: boolean
      skipExistingMessage: string
      networkErrorMessage: string
      okColumnsMessage: (cols: number) => string
      onConflict?: (message: string, existingJobId?: string) => void
      onFinished?: () => void
    }) => void
    cancel: () => void
  }
}

const AIJobsContext = createContext<AIJobsContextValue | null>(null)

function asString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function jobQuestionPreview(kind: AIJobKind, req: unknown): string {
  if (!req || typeof req !== 'object') return kind
  const record = req as Record<string, unknown>
  if (kind === 'describe') {
    const schema = asString(record.schema)
    const table = asString(record.table)
    const target = [schema, table].filter(Boolean).join('.')
    return target || kind
  }
  if (kind === 'describe_batch') {
    const tables = record.tables
    if (Array.isArray(tables)) {
      return `${tables.length} tables`
    }
  }
  if (kind === 'embed_metadata') {
    return 'Embedding refresh'
  }
  const q = asString(record.question)
  if (q.length <= 80) return q
  return `${q.slice(0, 77)}…`
}

export function trackedJobFromAIJob(job: AIJob): TrackedAIJob {
  const questionPreview =
    job.request_json && typeof job.request_json === 'object'
      ? jobQuestionPreview(job.kind, job.request_json)
      : undefined
  return { ...job, questionPreview }
}

function parseResult<TResult>(job: AIJob): TResult | null {
  if (!job.result_json) return null
  if (typeof job.result_json === 'object') return job.result_json as TResult
  return null
}

export function AIJobsProvider({ children }: { children: ReactNode }) {
  const sessionId = useMemo(() => getAIClientSessionId(), [])
  const [jobs, setJobs] = useState<TrackedAIJob[]>([])
  const [expanded, setExpanded] = useState(false)
  const [minimized, setMinimized] = useState(true)
  const [bulkEntries, setBulkEntries] = useState<BulkEntry[]>([])
  const [bulkRunning, setBulkRunning] = useState(false)
  const [bulkSummary, setBulkSummary] = useState<BulkDescribeSummary | null>(null)
  const callbacksRef = useRef<Map<string, JobWaiterHandle>>(new Map())
  const pollingIdsRef = useRef<Set<string>>(new Set())
  const pollLoopRef = useRef<number | null>(null)
  const bulkCancelRef = useRef(false)
  const bulkBatchJobIdRef = useRef<string | null>(null)
  const bulkEntriesRef = useRef<BulkEntry[]>([])
  const runJobRef = useRef<AIJobsContextValue['runJob'] | null>(null)

  const applyBulkProgressFromJob = useCallback((job: AIJob, queue: BulkEntry[]): BulkEntry[] => {
    const progress = job.progress_json as DescribeBatchJobProgress | null | undefined
    if (!progress) {
      const msg = job.phase_message?.trim()
      if (!msg) return queue
      return queue.map((entry) =>
        entry.status === 'running' ? { ...entry, message: msg } : entry,
      )
    }
    const completed = new Set(progress.completed ?? [])
    const curSchema = progress.current_schema?.trim() ?? ''
    const curTable = progress.current_table?.trim() ?? ''
    return queue.map((entry) => {
      if (entry.status === 'skipped') return entry
      const key = `${entry.schema}.${entry.table}`
      if (completed.has(key)) {
        return { ...entry, status: 'ok', message: job.phase_message || entry.message }
      }
      if (curSchema === entry.schema && curTable === entry.table) {
        return { ...entry, status: 'running', message: job.phase_message || '' }
      }
      if (entry.status === 'running' && !(curSchema === entry.schema && curTable === entry.table)) {
        return { ...entry, status: 'pending', message: undefined }
      }
      if (entry.status === 'pending') return entry
      return entry
    })
  }, [])

  const upsertJob = useCallback((job: TrackedAIJob) => {
    if (!isValidJobId(job.id)) return
    setJobs((prev) => {
      const idx = prev.findIndex((j) => j.id === job.id)
      if (idx === -1) return [job, ...prev].slice(0, 12)
      const next = [...prev]
      next[idx] = { ...next[idx], ...job }
      return next
    })
  }, [])

  const stopPollLoop = useCallback(() => {
    if (pollLoopRef.current != null) {
      window.clearInterval(pollLoopRef.current)
      pollLoopRef.current = null
    }
  }, [])

  const stopPolling = useCallback(
    (jobId: string) => {
      pollingIdsRef.current.delete(jobId)
      if (pollingIdsRef.current.size === 0) stopPollLoop()
    },
    [stopPollLoop],
  )

  const finishJob = useCallback((job: AIJob) => {
    stopPolling(job.id)
    const waiter = callbacksRef.current.get(job.id)
    if (!waiter) return
    callbacksRef.current.delete(job.id)
    if (job.status === 'succeeded') {
      const result = parseResult<unknown>(job)
      if (result != null) {
        waiter.settleComplete(result)
      } else {
        waiter.settleError('Job succeeded without result')
      }
    } else if (job.status === 'failed') {
      waiter.settleError(job.error_message || 'Job failed')
    } else if (job.status === 'cancelled') {
      waiter.settleError(job.phase_message || 'Job cancelled')
    }
  }, [stopPolling])

  const pollJob = useCallback(
    async (jobId: string) => {
      if (!isValidJobId(jobId)) {
        stopPolling(jobId)
        return
      }
      const { data, status, error } = await fetchJSON<AIJob>(`/api/ai/jobs/${encodeURIComponent(jobId)}`)
      if (error) {
        stopPolling(jobId)
        const waiter = callbacksRef.current.get(jobId)
        if (waiter) {
          callbacksRef.current.delete(jobId)
          waiter.settleError(error)
        }
        setMinimized(false)
        return
      }
      if (status === 404 || !data) {
        stopPolling(jobId)
        const waiter = callbacksRef.current.get(jobId)
        if (waiter) {
          callbacksRef.current.delete(jobId)
          waiter.settleDismiss()
        }
        return
      }
      upsertJob(trackedJobFromAIJob(data))
      if (bulkBatchJobIdRef.current === data.id && data.kind === 'describe_batch') {
        setBulkEntries((prev) => {
          const next = applyBulkProgressFromJob(data, prev)
          bulkEntriesRef.current = next
          return next
        })
      }
      if (TERMINAL.has(data.status)) {
        finishJob(data)
        if (data.status === 'failed' || data.status === 'succeeded') {
          setMinimized(false)
        }
      }
    },
    [applyBulkProgressFromJob, finishJob, stopPolling, upsertJob],
  )

  const runPollLoop = useCallback(async () => {
    const ids = [...pollingIdsRef.current]
    if (ids.length === 0) return
    await Promise.all(ids.map((id) => pollJob(id)))
  }, [pollJob])

  const ensurePollLoop = useCallback(() => {
    if (pollLoopRef.current != null) return
    pollLoopRef.current = window.setInterval(() => {
      void runPollLoop()
    }, POLL_MS)
  }, [runPollLoop])

  const startPolling = useCallback(
    (jobId: string) => {
      if (!isValidJobId(jobId)) return
      if (pollingIdsRef.current.has(jobId)) return
      pollingIdsRef.current.add(jobId)
      void pollJob(jobId)
      ensurePollLoop()
    },
    [ensurePollLoop, pollJob],
  )

  const resumeActiveJobs = useCallback(async () => {
    const { data, status, error } = await fetchJSON<{ jobs: AIJob[] }>(
      `/api/ai/jobs?client_session_id=${encodeURIComponent(sessionId)}&active=true`,
    )
    if (error) return
    if (status === 404 || !data?.jobs?.length) return
    for (const job of data.jobs) {
      if (!isValidJobId(job.id)) continue
      upsertJob(trackedJobFromAIJob(job))
      startPolling(job.id)
    }
    setMinimized(false)
  }, [sessionId, startPolling, upsertJob])

  useEffect(() => {
    void resumeActiveJobs()
    return () => {
      pollingIdsRef.current.clear()
      stopPollLoop()
      for (const waiter of callbacksRef.current.values()) {
        waiter.settleDismiss()
      }
      callbacksRef.current.clear()
    }
  }, [resumeActiveJobs, stopPollLoop])

  const runJob = useCallback(
    async <TRequest extends object, TResult = AIQueryResponse>(
      kind: AIJobKind,
      request: TRequest,
      callbacks?: JobCallbacks<TResult>,
    ) => {
      const body = {
        client_session_id: sessionId,
        kind,
        request,
      }
      const { data, status, error } = await fetchJSON<AIJob>('/api/ai/jobs', {
        method: 'POST',
        body: JSON.stringify(body),
      })
      if (error) {
        callbacks?.onError?.(error)
        return null
      }
      if (status === 404 || status === 405) {
        return 'fallback'
      }
      if (status < 200 || status >= 300 || !data || !isValidJobId(data.id)) {
        callbacks?.onError?.(jobEnqueueError(data, status))
        return null
      }
      upsertJob({ ...data, questionPreview: jobQuestionPreview(kind, request) })
      setMinimized(false)
      startPolling(data.id)
      return new Promise<TResult | null>((resolve) => {
        const waiter = createJobWaiter<TResult>(resolve, callbacks)
        callbacksRef.current.set(data.id, waiter)
      })
    },
    [sessionId, startPolling, upsertJob],
  )

  const dismissJob = useCallback((id: string) => {
    setJobs((prev) => prev.filter((j) => j.id !== id))
    stopPolling(id)
    const waiter = callbacksRef.current.get(id)
    if (waiter) {
      callbacksRef.current.delete(id)
      waiter.settleDismiss()
    }
  }, [stopPolling])

  const cancelJob = useCallback(
    async (id: string) => {
      const { data, status, error } = await fetchJSON<AIJob>(`/api/ai/jobs/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (error) return false
      if (status === 404 || status === 405) return false
      if (data) {
        upsertJob(trackedJobFromAIJob(data))
        finishJob(data)
      } else {
        await pollJob(id)
      }
      return true
    },
    [finishJob, pollJob, upsertJob],
  )

  runJobRef.current = runJob

  const cancelAllActiveJobs = useCallback(async () => {
    const { data, status, error } = await fetchJSON<{ cancelled: number }>('/api/ai/jobs/cancel-active', {
      method: 'POST',
      body: JSON.stringify({ client_session_id: sessionId }),
    })
    if (error) return 0
    if (status === 404 || status === 405 || !data) return 0
    const activeIds = jobs.filter(jobIsActive).map((j) => j.id)
    for (const id of activeIds) {
      await pollJob(id)
    }
    return data.cancelled ?? 0
  }, [jobs, pollJob, sessionId])

  const listStaleJobs = useCallback(
    async (olderMinutes = 15) => {
      const { data, status, error } = await fetchJSON<AIJobListResponse>(
        `/api/ai/jobs/stale?client_session_id=${encodeURIComponent(sessionId)}&older_minutes=${olderMinutes}`,
      )
      if (error) return []
      if (status === 404 || status === 405 || !data?.jobs) return []
      return data.jobs
    },
    [sessionId],
  )

  const cancelJobIds = useCallback(async (ids: string[]) => {
    if (!ids.length) return 0
    const { data, status, error } = await fetchJSON<{ cancelled: number }>('/api/ai/jobs/cancel-batch', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    })
    if (error) return 0
    if (status === 404 || status === 405 || !data) return 0
    for (const id of ids) {
      await pollJob(id)
    }
    return data.cancelled ?? 0
  }, [pollJob])

  const cancelBulkDescribe = useCallback(() => {
    bulkCancelRef.current = true
    const batchId = bulkBatchJobIdRef.current
    if (batchId) {
      bulkBatchJobIdRef.current = null
      void cancelJob(batchId)
    }
  }, [cancelJob])

  const startBulkDescribe = useCallback(
    (opts: {
      datasourceId: string
      targets: BulkDescribeTarget[]
      sampleSize: number
      skipExisting: boolean
      skipExistingMessage: string
      networkErrorMessage: string
      okColumnsMessage: (cols: number) => string
      onConflict?: (message: string, existingJobId?: string) => void
      onFinished?: () => void
    }) => {
      const { datasourceId, targets, sampleSize, skipExisting, onConflict, onFinished } = opts
      if (!datasourceId || targets.length === 0) return

      bulkCancelRef.current = false
      setBulkRunning(true)
      setBulkSummary(null)
      setMinimized(false)

      const queue: BulkEntry[] = targets.map((row) => {
        if (skipExisting && row.description) {
          return {
            schema: row.schema_name,
            table: row.table_name,
            status: 'skipped',
            message: opts.skipExistingMessage,
          }
        }
        return { schema: row.schema_name, table: row.table_name, status: 'pending' }
      })
      setBulkEntries(queue)
      bulkEntriesRef.current = queue

      const applyBatchResult = (batch: DescribeBatchResult) => {
        let ok = 0
        let errCount = 0
        let skipped = 0
        for (let i = 0; i < queue.length; i++) {
          const entry = queue[i]
          if (!entry || entry.status === 'skipped') {
            skipped++
            continue
          }
          const match = batch.entries.find(
            (e) => e.schema === entry.schema && e.table === entry.table,
          )
          if (!match) continue
          if (match.status === 'ok') {
            const cols = match.result?.columns?.length ?? 0
            queue[i] = {
              schema: entry.schema,
              table: entry.table,
              status: 'ok',
              message: opts.okColumnsMessage(cols),
            }
            ok++
          } else if (match.status === 'skipped') {
            queue[i] = {
              schema: entry.schema,
              table: entry.table,
              status: 'skipped',
              message: match.message || opts.skipExistingMessage,
            }
            skipped++
          } else {
            queue[i] = {
              schema: entry.schema,
              table: entry.table,
              status: 'error',
              message: match.message || opts.networkErrorMessage,
            }
            errCount++
          }
        }
        setBulkEntries([...queue])
        setBulkSummary({ ok, error: errCount, skipped })
      }

      const runSequential = async () => {
        let ok = 0
        let errCount = 0
        let skipped = queue.filter((q) => q.status === 'skipped').length
        const runOne = runJobRef.current
        if (!runOne) {
          setBulkRunning(false)
          return
        }

        for (let i = 0; i < targets.length; i++) {
          if (bulkCancelRef.current) break
          const row = targets[i]
          const entry = queue[i]
          if (!row || !entry || entry.status === 'skipped') continue

          const schema = row.schema_name
          const table = row.table_name
          queue[i] = { schema, table, status: 'running' }
          setBulkEntries([...queue])

          try {
            const request = {
              datasource_id: datasourceId,
              schema,
              table,
              sample_size: sampleSize,
              auto_apply: true,
            }
            let data = await runOne<typeof request, DescribeResult>('describe', request)
            if (data === 'fallback') {
              data = await runMetadataDescribeDirect(request)
            }
            if (!data) {
              queue[i] = { schema, table, status: 'error', message: opts.networkErrorMessage }
              errCount++
            } else {
              const cols = data.columns?.length ?? 0
              queue[i] = { schema, table, status: 'ok', message: opts.okColumnsMessage(cols) }
              ok++
            }
          } catch (err) {
            queue[i] = {
              schema,
              table,
              status: 'error',
              message: err instanceof Error ? err.message : opts.networkErrorMessage,
            }
            errCount++
          }
          setBulkEntries([...queue])
        }

        setBulkSummary({ ok, error: errCount, skipped })
      }

      void (async () => {
        bulkBatchJobIdRef.current = null
        const batchRequest = {
          datasource_id: datasourceId,
          tables: targets.map((row) => ({
            schema: row.schema_name,
            table: row.table_name,
          })),
          sample_size: sampleSize,
          auto_apply: true,
          skip_existing: skipExisting,
        }

        const { data: enqueued, status, error } = await fetchJSON<AIJob | DescribeBatchConflictBody>(
          '/api/ai/jobs',
          {
            method: 'POST',
            body: JSON.stringify({
              client_session_id: sessionId,
              kind: 'describe_batch',
              request: batchRequest,
            }),
          },
        )
        if (error) {
          setBulkEntries((prev) => {
            const next = prev.map((entry) =>
              entry.status === 'running' ? { ...entry, status: 'error' as const, message: error } : entry,
            )
            bulkEntriesRef.current = next
            return next
          })
          setBulkRunning(false)
          bulkBatchJobIdRef.current = null
          opts.onFinished?.()
          return
        }

        if (status === 409 && enqueued && typeof enqueued === 'object' && 'error' in enqueued) {
          const conflict = enqueued as DescribeBatchConflictBody
          onConflict?.(conflict.error, conflict.existing_job_id)
          setBulkRunning(false)
          onFinished?.()
          return
        }

        if (
          status >= 200 &&
          status < 300 &&
          enqueued &&
          isValidJobId((enqueued as AIJob).id)
        ) {
          const job = enqueued as AIJob
          bulkBatchJobIdRef.current = job.id
          upsertJob({
            ...job,
            questionPreview: jobQuestionPreview('describe_batch', batchRequest),
          })
          startPolling(job.id)

          const batchResult = await new Promise<DescribeBatchResult | null>((resolve) => {
            const waiter = createJobWaiter<DescribeBatchResult>(resolve)
            callbacksRef.current.set(job.id, waiter)
          })
          bulkBatchJobIdRef.current = null

          if (bulkCancelRef.current) {
            setBulkRunning(false)
            onFinished?.()
            return
          }

          if (batchResult) {
            applyBatchResult(batchResult)
          } else {
            await runSequential()
          }
        } else {
          await runSequential()
        }

        setBulkRunning(false)
        onFinished?.()
      })()
    },
    [sessionId, startPolling, upsertJob],
  )

  const value = useMemo(
    () => ({
      sessionId,
      jobs,
      expanded,
      setExpanded,
      minimized,
      setMinimized,
      dismissJob,
      cancelJob,
      cancelAllActiveJobs,
      listStaleJobs,
      cancelJobIds,
      runJob,
      bulkDescribe: {
        running: bulkRunning,
        entries: bulkEntries,
        summary: bulkSummary,
        start: startBulkDescribe,
        cancel: cancelBulkDescribe,
      },
    }),
    [
      sessionId,
      jobs,
      expanded,
      minimized,
      dismissJob,
      cancelJob,
      cancelAllActiveJobs,
      listStaleJobs,
      cancelJobIds,
      runJob,
      bulkRunning,
      bulkEntries,
      bulkSummary,
      startBulkDescribe,
      cancelBulkDescribe,
    ],
  )

  return <AIJobsContext.Provider value={value}>{children}</AIJobsContext.Provider>
}

export function useAIJobs(): AIJobsContextValue {
  const ctx = useContext(AIJobsContext)
  if (!ctx) {
    throw new Error('useAIJobs must be used within AIJobsProvider')
  }
  return ctx
}
