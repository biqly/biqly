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
import type { AIJob, AIJobKind, AIJobListResponse, AIQueryResponse } from '../types/ai'
import type { DescribeBatchResult, DescribeResult } from '../api/metadataDescribe'
import { runMetadataDescribeDirect } from '../api/metadataDescribe'
import type { BulkEntry } from '../components/metadata/bulkProgress'
import { getLocale } from '../i18n'
import { getAIClientSessionId } from '../utils/aiSession'

const POLL_MS = 1200
const TERMINAL = new Set(['succeeded', 'failed', 'cancelled'])

function jobIsActive(job: AIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

export type TrackedAIJob = AIJob & {
  questionPreview?: string
}

type JobCallbacks<TResult = unknown> = {
  onComplete?: (result: TResult) => void
  onError?: (message: string) => void
}

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
      onFinished?: () => void
    }) => void
    cancel: () => void
  }
}

const AIJobsContext = createContext<AIJobsContextValue | null>(null)

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<{ data: T | null; status: number }> {
  const res = await fetch(url, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Locale': getLocale(),
      ...(init?.headers ?? {}),
    },
  })
  const text = await res.text()
  if (!text) return { data: null, status: res.status }
  try {
    return { data: JSON.parse(text) as T, status: res.status }
  } catch {
    return { data: null, status: res.status }
  }
}

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
  const q = asString(record.question)
  if (q.length <= 80) return q
  return `${q.slice(0, 77)}…`
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
  const callbacksRef = useRef<Map<string, JobCallbacks<unknown>>>(new Map())
  const pollTimers = useRef<Map<string, number>>(new Map())
  const bulkCancelRef = useRef(false)
  const bulkBatchJobIdRef = useRef<string | null>(null)
  const runJobRef = useRef<AIJobsContextValue['runJob'] | null>(null)

  const upsertJob = useCallback((job: TrackedAIJob) => {
    setJobs((prev) => {
      const idx = prev.findIndex((j) => j.id === job.id)
      if (idx === -1) return [job, ...prev].slice(0, 12)
      const next = [...prev]
      next[idx] = { ...next[idx], ...job }
      return next
    })
  }, [])

  const finishJob = useCallback((job: AIJob) => {
    const cbs = callbacksRef.current.get(job.id)
    callbacksRef.current.delete(job.id)
    const timer = pollTimers.current.get(job.id)
    if (timer != null) {
      window.clearInterval(timer)
      pollTimers.current.delete(job.id)
    }
    if (job.status === 'succeeded') {
      const result = parseResult<unknown>(job)
      if (result) cbs?.onComplete?.(result)
    } else if (job.status === 'failed') {
      cbs?.onError?.(job.error_message || 'Job failed')
    } else if (job.status === 'cancelled') {
      cbs?.onError?.(job.phase_message || 'Job cancelled')
    }
  }, [])

  const pollJob = useCallback(
    async (jobId: string) => {
      const { data, status } = await fetchJSON<AIJob>(`/api/ai/jobs/${encodeURIComponent(jobId)}`)
      if (status === 404 || !data) return
      const preview =
        data.request_json && typeof data.request_json === 'object'
          ? jobQuestionPreview(data.kind, data.request_json)
          : undefined
      upsertJob({ ...data, questionPreview: preview })
      if (TERMINAL.has(data.status)) {
        finishJob(data)
        if (data.status === 'failed' || data.status === 'succeeded') {
          setMinimized(false)
        }
      }
    },
    [finishJob, upsertJob],
  )

  const startPolling = useCallback(
    (jobId: string) => {
      if (pollTimers.current.has(jobId)) return
      void pollJob(jobId)
      const id = window.setInterval(() => void pollJob(jobId), POLL_MS)
      pollTimers.current.set(jobId, id)
    },
    [pollJob],
  )

  const resumeActiveJobs = useCallback(async () => {
    const { data, status } = await fetchJSON<{ jobs: AIJob[] }>(
      `/api/ai/jobs?client_session_id=${encodeURIComponent(sessionId)}&active=true`,
    )
    if (status === 404 || !data?.jobs?.length) return
    for (const job of data.jobs) {
      const preview =
        job.request_json && typeof job.request_json === 'object'
          ? jobQuestionPreview(job.kind, job.request_json)
          : undefined
      upsertJob({ ...job, questionPreview: preview })
      startPolling(job.id)
    }
    setMinimized(false)
  }, [sessionId, startPolling, upsertJob])

  useEffect(() => {
    void resumeActiveJobs()
    return () => {
      for (const id of pollTimers.current.values()) window.clearInterval(id)
      pollTimers.current.clear()
    }
  }, [resumeActiveJobs])

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
      const { data, status } = await fetchJSON<AIJob>('/api/ai/jobs', {
        method: 'POST',
        body: JSON.stringify(body),
      })
      if (status === 404 || status === 405) {
        return 'fallback'
      }
      if (!data) {
        callbacks?.onError?.('Failed to enqueue AI job')
        return null
      }
      upsertJob({ ...data, questionPreview: jobQuestionPreview(kind, request) })
      setMinimized(false)
      startPolling(data.id)
      return new Promise<TResult | null>((resolve) => {
        callbacksRef.current.set(data.id, {
          onComplete: (result) => {
            const typed = result as TResult
            callbacks?.onComplete?.(typed)
            resolve(typed)
          },
          onError: (message) => {
            callbacks?.onError?.(message)
            resolve(null)
          },
        })
      })
    },
    [sessionId, startPolling, upsertJob],
  )

  const dismissJob = useCallback((id: string) => {
    setJobs((prev) => prev.filter((j) => j.id !== id))
    const timer = pollTimers.current.get(id)
    if (timer != null) {
      window.clearInterval(timer)
      pollTimers.current.delete(id)
    }
    callbacksRef.current.delete(id)
  }, [])

  const cancelJob = useCallback(
    async (id: string) => {
      const { data, status } = await fetchJSON<AIJob>(`/api/ai/jobs/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (status === 404 || status === 405) return false
      if (data) {
        const preview =
          data.request_json && typeof data.request_json === 'object'
            ? jobQuestionPreview(data.kind, data.request_json)
            : undefined
        upsertJob({ ...data, questionPreview: preview })
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
    const { data, status } = await fetchJSON<{ cancelled: number }>('/api/ai/jobs/cancel-active', {
      method: 'POST',
      body: JSON.stringify({ client_session_id: sessionId }),
    })
    if (status === 404 || status === 405 || !data) return 0
    const activeIds = jobs.filter(jobIsActive).map((j) => j.id)
    for (const id of activeIds) {
      await pollJob(id)
    }
    return data.cancelled ?? 0
  }, [jobs, pollJob, sessionId])

  const listStaleJobs = useCallback(
    async (olderMinutes = 15) => {
      const { data, status } = await fetchJSON<AIJobListResponse>(
        `/api/ai/jobs/stale?client_session_id=${encodeURIComponent(sessionId)}&older_minutes=${olderMinutes}`,
      )
      if (status === 404 || status === 405 || !data?.jobs) return []
      return data.jobs
    },
    [sessionId],
  )

  const cancelJobIds = useCallback(async (ids: string[]) => {
    if (!ids.length) return 0
    const { data, status } = await fetchJSON<{ cancelled: number }>('/api/ai/jobs/cancel-batch', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    })
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
      onFinished?: () => void
    }) => {
      const { datasourceId, targets, sampleSize, skipExisting, onFinished } = opts
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

        const { data: enqueued, status } = await fetchJSON<AIJob>('/api/ai/jobs', {
          method: 'POST',
          body: JSON.stringify({
            client_session_id: sessionId,
            kind: 'describe_batch',
            request: batchRequest,
          }),
        })

        if (status !== 404 && status !== 405 && enqueued) {
          bulkBatchJobIdRef.current = enqueued.id
          upsertJob({
            ...enqueued,
            questionPreview: jobQuestionPreview('describe_batch', batchRequest),
          })
          startPolling(enqueued.id)

          for (let i = 0; i < queue.length; i++) {
            const entry = queue[i]
            if (entry?.status === 'pending') {
              queue[i] = {
                schema: entry.schema,
                table: entry.table,
                status: 'running',
                message: enqueued.phase_message || '',
              }
            }
          }
          setBulkEntries([...queue])

          const batchResult = await new Promise<DescribeBatchResult | null>((resolve) => {
            callbacksRef.current.set(enqueued.id, {
              onComplete: (result) => resolve(result as DescribeBatchResult),
              onError: () => resolve(null),
            })
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
