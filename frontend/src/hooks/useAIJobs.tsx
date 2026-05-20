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
import type { AIJob, AIJobKind, AIQueryRequest, AIQueryResponse } from '../types/ai'
import { getLocale } from '../i18n'
import { getAIClientSessionId } from '../utils/aiSession'

const POLL_MS = 1200
const TERMINAL = new Set(['succeeded', 'failed', 'cancelled'])

export type TrackedAIJob = AIJob & {
  questionPreview?: string
}

type JobCallbacks = {
  onComplete?: (result: AIQueryResponse) => void
  onError?: (message: string) => void
}

type AIJobsContextValue = {
  sessionId: string
  jobs: TrackedAIJob[]
  expanded: boolean
  setExpanded: (v: boolean) => void
  minimized: boolean
  setMinimized: (v: boolean) => void
  dismissJob: (id: string) => void
  runJob: (
    kind: AIJobKind,
    request: AIQueryRequest,
    callbacks?: JobCallbacks,
  ) => Promise<AIQueryResponse | 'fallback' | null>
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

function questionPreview(req: AIQueryRequest): string {
  const q = req.question?.trim() ?? ''
  if (q.length <= 80) return q
  return `${q.slice(0, 77)}…`
}

function parseResult(job: AIJob): AIQueryResponse | null {
  if (!job.result_json) return null
  if (typeof job.result_json === 'object') return job.result_json as AIQueryResponse
  return null
}

export function AIJobsProvider({ children }: { children: ReactNode }) {
  const sessionId = useMemo(() => getAIClientSessionId(), [])
  const [jobs, setJobs] = useState<TrackedAIJob[]>([])
  const [expanded, setExpanded] = useState(false)
  const [minimized, setMinimized] = useState(true)
  const callbacksRef = useRef<Map<string, JobCallbacks>>(new Map())
  const pollTimers = useRef<Map<string, number>>(new Map())

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
      const result = parseResult(job)
      if (result) cbs?.onComplete?.(result)
    } else if (job.status === 'failed') {
      cbs?.onError?.(job.error_message || 'Job failed')
    }
  }, [])

  const pollJob = useCallback(
    async (jobId: string) => {
      const { data, status } = await fetchJSON<AIJob>(`/api/ai/jobs/${encodeURIComponent(jobId)}`)
      if (status === 404 || !data) return
      const preview =
        data.request_json && typeof data.request_json === 'object'
          ? questionPreview(data.request_json as AIQueryRequest)
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
          ? questionPreview(job.request_json as AIQueryRequest)
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
    async (kind: AIJobKind, request: AIQueryRequest, callbacks?: JobCallbacks) => {
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
      upsertJob({ ...data, questionPreview: questionPreview(request) })
      setMinimized(false)
      startPolling(data.id)
      return new Promise<AIQueryResponse | null>((resolve) => {
        callbacksRef.current.set(data.id, {
          onComplete: (result) => {
            callbacks?.onComplete?.(result)
            resolve(result)
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

  const value = useMemo(
    () => ({
      sessionId,
      jobs,
      expanded,
      setExpanded,
      minimized,
      setMinimized,
      dismissJob,
      runJob,
    }),
    [sessionId, jobs, expanded, minimized, dismissJob, runJob],
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
