/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'

import { fetchJSON, type FetchJSONResult } from '../api/apiClient'
import { useAuth } from '../components/auth/AuthProvider'
import type { BulkEntry } from '../components/metadata/bulkProgress'
import type { AIJob, AIJobKind, AIJobListResponse, AIQueryResponse } from '../types/ai'
import { getAIClientSessionId } from '../utils/aiSession'
import { buildInitialBulkQueue, runBulkDescribeEnqueue } from './bulkDescribeRunner'
import { createJobWaiter, type JobCallbacks, type JobWaiterHandle } from './jobWaiter'
export { fetchJSON }
export type { FetchJSONResult }

const POLL_MS = 1200
const TERMINAL = new Set(['succeeded', 'failed', 'cancelled'])

function isValidJobId(id: unknown): id is string {
  if (typeof id !== 'string') {
    return false
  }
  const trimmed = id.trim()
  return trimmed.length > 0 && trimmed !== 'undefined' && trimmed !== 'null'
}

function jobEnqueueError(data: unknown, status: number): string {
  if (data && typeof data === 'object') {
    const err = (data as Record<string, unknown>).error
    if (typeof err === 'string' && err.trim()) {
      return err.trim()
    }
  }
  return `Failed to enqueue AI job (${status})`
}

export function jobIsActive(job: AIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

export type TrackedAIJob = AIJob & {
  questionPreview?: string
  /** Client-side per-phase durations in ms, keyed by phase name. */
  phaseTimings?: Record<string, number>
  /** Epoch ms when the current phase was first observed client-side. */
  phaseEnteredAt?: number
}

export type { JobCallbacks } from './jobWaiter'

export interface BulkDescribeSummary {
  ok: number
  error: number
  skipped: number
}

export interface BulkDescribeTarget {
  schema_name: string
  table_name: string
  description: string | null
}

interface AIJobsContextValue {
  sessionId: string
  jobs: TrackedAIJob[]
  expanded: boolean
  setExpanded: (v: boolean) => void
  minimized: boolean
  setMinimized: (v: boolean) => void
  dismissJob: (id: string) => void
  dismissFinishedJobs: () => void
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
  if (!req || typeof req !== 'object') {
    return kind
  }
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
  if (q.length <= 80) {
    return q
  }
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
  if (!job.result_json) {
    return null
  }
  if (typeof job.result_json === 'object') {
    return job.result_json as TResult
  }
  return null
}

export function AIJobsProvider({ children }: { children: ReactNode }) {
  const sessionId = useMemo(() => getAIClientSessionId(), [])
  const { accessToken } = useAuth()
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
  const isMountedRef = useRef(true)
  const bulkRunIdRef = useRef(0)
  const resumedRef = useRef(false)

  const applyBulkProgressFromJob = useCallback((job: AIJob, queue: BulkEntry[]): BulkEntry[] => {
    const progress = job.progress_json
    if (!progress) {
      const msg = job.phase_message.trim()
      if (!msg) {
        return queue
      }
      return queue.map((entry) => (entry.status === 'running' ? { ...entry, message: msg } : entry))
    }
    const completed = new Set(progress.completed ?? [])
    const curSchema = progress.current_schema?.trim() ?? ''
    const curTable = progress.current_table?.trim() ?? ''
    return queue.map((entry) => {
      if (entry.status === 'skipped') {
        return entry
      }
      const key = `${entry.schema}.${entry.table}`
      if (completed.has(key)) {
        return { ...entry, status: 'ok', message: `described ${key}` }
      }
      if (curSchema === entry.schema && curTable === entry.table) {
        return { ...entry, status: 'running', message: `describing ${key}` }
      }
      if (entry.status === 'running' && !(curSchema === entry.schema && curTable === entry.table)) {
        return { ...entry, status: 'pending', message: undefined }
      }
      if (entry.status === 'pending') {
        return entry
      }
      return entry
    })
  }, [])

  const upsertJob = useCallback((job: TrackedAIJob) => {
    if (!isValidJobId(job.id)) {
      return
    }
    const now = Date.now()
    setJobs((prev) => {
      const idx = prev.findIndex((j) => j.id === job.id)
      if (idx === -1) {
        return [{ phaseTimings: {}, phaseEnteredAt: now, ...job }, ...prev].slice(0, 12)
      }
      const existing = prev[idx]
      if (!existing) {
        return prev
      }
      const timings = { ...(existing.phaseTimings ?? {}) }
      let enteredAt = existing.phaseEnteredAt ?? now
      if (existing.phase !== job.phase) {
        timings[existing.phase] = (timings[existing.phase] ?? 0) + (now - enteredAt)
        enteredAt = now
      }
      if (TERMINAL.has(job.status) && timings[job.phase] === undefined) {
        timings[job.phase] = now - enteredAt
      }
      const next = [...prev]
      next[idx] = { ...existing, ...job, phaseTimings: timings, phaseEnteredAt: enteredAt }
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
      if (pollingIdsRef.current.size === 0) {
        stopPollLoop()
      }
    },
    [stopPollLoop],
  )

  const finishJob = useCallback(
    (job: AIJob) => {
      stopPolling(job.id)
      const waiter = callbacksRef.current.get(job.id)
      if (!waiter) {
        return
      }
      callbacksRef.current.delete(job.id)
      if (job.status === 'succeeded') {
        const result = parseResult<unknown>(job)
        if (result != null) {
          waiter.settleComplete(result)
        } else {
          waiter.settleError('Job succeeded without result')
        }
      } else if (job.status === 'failed') {
        waiter.settleError(job.error_message ?? 'Job failed')
      } else if (job.status === 'cancelled') {
        waiter.settleError(job.phase_message || 'Job cancelled')
      }
    },
    [stopPolling],
  )

  const pollJob = useCallback(
    async (jobId: string) => {
      if (!isValidJobId(jobId)) {
        stopPolling(jobId)
        return
      }
      const { data, status, error } = await fetchJSON<AIJob>(
        `/api/ai/jobs/${encodeURIComponent(jobId)}`,
      )
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
        // Best practice: only interrupt the user for failures. Successful
        // results already surface in the requesting UI; the tray FAB reflects
        // the completed state without stealing focus.
        if (data.status === 'failed') {
          setMinimized(false)
        }
      }
    },
    [applyBulkProgressFromJob, finishJob, stopPolling, upsertJob],
  )

  const runPollLoop = useCallback(async () => {
    const ids = [...pollingIdsRef.current]
    if (ids.length === 0) {
      return
    }
    await Promise.all(ids.map((id) => pollJob(id)))
  }, [pollJob])

  const ensurePollLoop = useCallback(() => {
    if (pollLoopRef.current != null) {
      return
    }
    pollLoopRef.current = window.setInterval(() => {
      void runPollLoop()
    }, POLL_MS)
  }, [runPollLoop])

  const startPolling = useCallback(
    (jobId: string) => {
      if (!isValidJobId(jobId)) {
        return
      }
      if (pollingIdsRef.current.has(jobId)) {
        return
      }
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
    if (error) {
      return
    }
    if (status === 404 || !data?.jobs.length) {
      return
    }
    for (const job of data.jobs) {
      if (!isValidJobId(job.id)) {
        continue
      }
      upsertJob(trackedJobFromAIJob(job))
      startPolling(job.id)
    }
  }, [sessionId, startPolling, upsertJob])

  useEffect(() => {
    isMountedRef.current = true
    const pollingIds = pollingIdsRef
    const callbacks = callbacksRef
    return () => {
      isMountedRef.current = false
      pollingIds.current.clear()
      stopPollLoop()
      for (const waiter of callbacks.current.values()) {
        waiter.settleDismiss()
      }
      callbacks.current.clear()
    }
  }, [stopPollLoop])

  // Resume active jobs only once an access token is available. The provider
  // mounts before AuthProvider hydrates the session, so firing the request
  // unauthenticated would return 401. Reset on sign-out so a later sign-in
  // resumes again.
  useEffect(() => {
    if (!accessToken) {
      resumedRef.current = false
      return
    }
    if (resumedRef.current) {
      return
    }
    resumedRef.current = true
    void resumeActiveJobs()
  }, [accessToken, resumeActiveJobs])

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
      startPolling(data.id)
      return new Promise<TResult | null>((resolve) => {
        const waiter = createJobWaiter<TResult>(resolve, callbacks)
        callbacksRef.current.set(data.id, waiter)
      })
    },
    [sessionId, startPolling, upsertJob],
  )

  const dismissJob = useCallback(
    (id: string) => {
      setJobs((prev) => prev.filter((j) => j.id !== id))
      stopPolling(id)
      const waiter = callbacksRef.current.get(id)
      if (waiter) {
        callbacksRef.current.delete(id)
        waiter.settleDismiss()
      }
    },
    [stopPolling],
  )

  const dismissFinishedJobs = useCallback(() => {
    setJobs((prev) => prev.filter((j) => !TERMINAL.has(j.status)))
  }, [])

  const cancelJob = useCallback(
    async (id: string) => {
      const { data, status, error } = await fetchJSON<AIJob>(
        `/api/ai/jobs/${encodeURIComponent(id)}`,
        {
          method: 'DELETE',
        },
      )
      if (error) {
        return false
      }
      if (status === 404 || status === 405) {
        return false
      }
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

  useEffect(() => {
    runJobRef.current = runJob
  }, [runJob])

  const cancelAllActiveJobs = useCallback(async () => {
    const { data, status, error } = await fetchJSON<{ cancelled: number }>(
      '/api/ai/jobs/cancel-active',
      {
        method: 'POST',
        body: JSON.stringify({ client_session_id: sessionId }),
      },
    )
    if (error) {
      return 0
    }
    if (status === 404 || status === 405 || !data) {
      return 0
    }
    const activeIds = jobs.filter(jobIsActive).map((j) => j.id)
    for (const id of activeIds) {
      await pollJob(id)
    }
    return data.cancelled
  }, [jobs, pollJob, sessionId])

  const listStaleJobs = useCallback(
    async (olderMinutes = 15) => {
      const { data, status, error } = await fetchJSON<AIJobListResponse>(
        `/api/ai/jobs/stale?client_session_id=${encodeURIComponent(sessionId)}&older_minutes=${olderMinutes}`,
      )
      if (error) {
        return []
      }
      if (status === 404 || status === 405 || !data?.jobs) {
        return []
      }
      return data.jobs
    },
    [sessionId],
  )

  const cancelJobIds = useCallback(
    async (ids: string[]) => {
      if (!ids.length) {
        return 0
      }
      const { data, status, error } = await fetchJSON<{ cancelled: number }>(
        '/api/ai/jobs/cancel-batch',
        {
          method: 'POST',
          body: JSON.stringify({ ids }),
        },
      )
      if (error) {
        return 0
      }
      if (status === 404 || status === 405 || !data) {
        return 0
      }
      for (const id of ids) {
        await pollJob(id)
      }
      return data.cancelled
    },
    [pollJob],
  )

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
      if (!datasourceId || targets.length === 0) {
        return
      }

      bulkCancelRef.current = false
      const runId = ++bulkRunIdRef.current
      setBulkRunning(true)
      setBulkSummary(null)

      const queue = buildInitialBulkQueue(targets, skipExisting, opts.skipExistingMessage)
      setBulkEntries(queue)
      bulkEntriesRef.current = queue

      const isStale = () => !isMountedRef.current || bulkRunIdRef.current !== runId

      void runBulkDescribeEnqueue({
        sessionId,
        datasourceId,
        targets,
        sampleSize,
        skipExisting,
        skipExistingMessage: opts.skipExistingMessage,
        networkErrorMessage: opts.networkErrorMessage,
        okColumnsMessage: opts.okColumnsMessage,
        queue,
        isStale,
        isCancelled: () => bulkCancelRef.current,
        onConflict,
        setBulkEntries: (updater) => {
          if (typeof updater === 'function') {
            setBulkEntries((prev) => {
              const next = updater(prev)
              bulkEntriesRef.current = next
              return next
            })
          } else {
            setBulkEntries(updater)
            bulkEntriesRef.current = updater
          }
        },
        setBulkSummary,
        setBulkRunning,
        setBulkBatchJobId: (id) => {
          bulkBatchJobIdRef.current = id
        },
        upsertJob,
        startPolling,
        registerWaiter: (jobId, waiter) => {
          callbacksRef.current.set(jobId, waiter)
        },
        runOne: <TReq extends object, TRes>(kind: 'describe', request: TReq) => {
          const runOne = runJobRef.current
          if (!runOne) {
            return Promise.resolve<TRes | 'fallback' | null>(null)
          }
          return runOne<TReq, TRes>(kind, request)
        },
        onFinished,
      })
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
      dismissFinishedJobs,
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
      dismissFinishedJobs,
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
