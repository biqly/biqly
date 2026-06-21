import type { DescribeBatchConflictBody } from '../api/describeBatchConflict'
import type { BulkEntry } from '../components/metadata/bulkProgressUtils'
import type { AIJob } from '../types/ai'
import type { DescribeBatchResult } from '../types/metadata'
import { parseDescribeBatchResult } from '../utils/parseJobResults'
import { applyBatchResultToQueue, runSequentialBulkDescribe } from './bulkDescribeRunner'
import type { JobWaiterHandle } from './jobWaiter'
import { createJobWaiter } from './jobWaiter'
import {
  type BulkDescribeSummary,
  type BulkDescribeTarget,
  type DescribeBatchJobRequest,
  jobQuestionPreview,
  type TrackedAIJob,
} from './useAIJobsUtils'

interface BulkEnqueueOpts {
  sessionId: string
  datasourceId: string
  targets: BulkDescribeTarget[]
  sampleSize: number
  locale?: string
  skipExisting: boolean
  skipExistingMessage: string
  networkErrorMessage: string
  okColumnsMessage: (cols: number) => string
  queue: BulkEntry[]
  isStale: () => boolean
  isCancelled: () => boolean
  onConflict?: (message: string, existingJobId?: string) => void
  setBulkEntries: (updater: BulkEntry[] | ((prev: BulkEntry[]) => BulkEntry[])) => void
  setBulkSummary: (summary: BulkDescribeSummary) => void
  setBulkRunning: (running: boolean) => void
  setBulkBatchJobId: (id: string | null) => void
  upsertJob: (job: TrackedAIJob) => void
  startPolling: (id: string) => void
  registerWaiter: (jobId: string, waiter: JobWaiterHandle) => void
  runOne: <TReq extends object, TRes>(
    kind: 'describe',
    request: TReq,
  ) => Promise<TRes | 'fallback' | null>
  onFinished?: () => void
}

export function failBulkEnqueue(opts: BulkEnqueueOpts, error: string): void {
  opts.setBulkEntries((prev) =>
    prev.map((entry) =>
      entry.status === 'running' ? { ...entry, status: 'error' as const, message: error } : entry,
    ),
  )
  opts.setBulkRunning(false)
  opts.setBulkBatchJobId(null)
  opts.onFinished?.()
}

export function handleBulkDescribeConflict(
  opts: BulkEnqueueOpts,
  conflict: DescribeBatchConflictBody,
): void {
  opts.onConflict?.(conflict.error, conflict.existing_job_id)
  opts.setBulkRunning(false)
  opts.onFinished?.()
}

async function runSequentialFallback(opts: BulkEnqueueOpts): Promise<BulkDescribeSummary> {
  return runSequentialBulkDescribe({
    targets: opts.targets,
    queue: opts.queue,
    datasourceId: opts.datasourceId,
    sampleSize: opts.sampleSize,
    networkErrorMessage: opts.networkErrorMessage,
    okColumnsMessage: opts.okColumnsMessage,
    isCancelled: opts.isCancelled,
    setBulkEntries: (entries) => opts.setBulkEntries(entries),
    runOne: opts.runOne,
  })
}

async function waitForBatchJobResult(
  opts: BulkEnqueueOpts,
  jobId: string,
): Promise<DescribeBatchResult | null> {
  return new Promise<DescribeBatchResult | null>((resolve) => {
    const waiter = createJobWaiter<DescribeBatchResult>(resolve, {
      parseResult: parseDescribeBatchResult,
    })
    opts.registerWaiter(jobId, waiter)
  })
}

export async function processEnqueuedBatchJob(
  opts: BulkEnqueueOpts,
  job: AIJob,
  batchRequest: DescribeBatchJobRequest,
): Promise<void> {
  opts.setBulkBatchJobId(job.id)
  opts.upsertJob({
    ...job,
    questionPreview: jobQuestionPreview('describe_batch', batchRequest),
  })
  opts.startPolling(job.id)

  const batchResult = await waitForBatchJobResult(opts, job.id)
  opts.setBulkBatchJobId(null)

  if (opts.isStale() || opts.isCancelled()) {
    opts.setBulkRunning(false)
    opts.onFinished?.()
    return
  }

  if (batchResult) {
    const summary = applyBatchResultToQueue(opts.queue, batchResult, {
      skipExistingMessage: opts.skipExistingMessage,
      networkErrorMessage: opts.networkErrorMessage,
      okColumnsMessage: opts.okColumnsMessage,
    })
    opts.setBulkEntries([...opts.queue])
    opts.setBulkSummary(summary)
    return
  }

  const summary = await runSequentialFallback(opts)
  opts.setBulkSummary(summary)
}

export async function finishBulkDescribeWithoutJob(opts: BulkEnqueueOpts): Promise<void> {
  if (opts.isStale()) {
    return
  }
  const summary = await runSequentialFallback(opts)
  opts.setBulkSummary(summary)
}
