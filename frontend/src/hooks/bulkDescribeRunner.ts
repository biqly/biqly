import { fetchJSON } from '../api/apiClient'
import type { DescribeBatchConflictBody } from '../api/describeBatchConflict'
import { runMetadataDescribeDirect } from '../api/metadataDescribe'
import type { BulkEntry } from '../components/metadata/bulkProgressUtils'
import type { AIJob } from '../types/ai'
import type { DescribeBatchResult, DescribeResult } from '../types/metadata'
import {
  failBulkEnqueue,
  finishBulkDescribeWithoutJob,
  handleBulkDescribeConflict,
  processEnqueuedBatchJob,
} from './bulkDescribeEnqueueHelpers'
import type { JobWaiterHandle } from './jobWaiter'
import {
  type BulkDescribeSummary,
  type BulkDescribeTarget,
  type DescribeBatchJobRequest,
  type DescribeJobRequest,
  type TrackedAIJob,
} from './useAIJobsUtils'

function isValidJobId(id: unknown): id is string {
  if (typeof id !== 'string') {
    return false
  }
  const trimmed = id.trim()
  return trimmed.length > 0 && trimmed !== 'undefined' && trimmed !== 'null'
}

function isDescribeBatchConflictBody(value: unknown): value is DescribeBatchConflictBody {
  if (!value || typeof value !== 'object') {
    return false
  }
  return 'error' in value
}

function isAIJob(value: unknown): value is AIJob {
  if (!value || typeof value !== 'object') {
    return false
  }
  return 'id' in value && isValidJobId((value as { id?: unknown }).id)
}

export function buildInitialBulkQueue(
  targets: BulkDescribeTarget[],
  skipExisting: boolean,
  skipExistingMessage: string,
): BulkEntry[] {
  return targets.map((row) => {
    if (skipExisting && row.description) {
      return {
        schema: row.schema_name,
        table: row.table_name,
        status: 'skipped',
        message: skipExistingMessage,
      }
    }
    return { schema: row.schema_name, table: row.table_name, status: 'pending' }
  })
}

export function applyBatchResultToQueue(
  queue: BulkEntry[],
  batch: DescribeBatchResult,
  messages: {
    skipExistingMessage: string
    networkErrorMessage: string
    okColumnsMessage: (cols: number) => string
  },
): BulkDescribeSummary {
  let ok = 0
  let errCount = 0
  let skipped = 0
  for (let i = 0; i < queue.length; i++) {
    const entry = queue[i]
    if (!entry || entry.status === 'skipped') {
      skipped++
      continue
    }
    const match = batch.entries.find((e) => e.schema === entry.schema && e.table === entry.table)
    if (!match) {
      continue
    }
    if (match.status === 'ok') {
      const cols = match.result?.columns.length ?? 0
      queue[i] = {
        schema: entry.schema,
        table: entry.table,
        status: 'ok',
        message: messages.okColumnsMessage(cols),
      }
      ok++
    } else if (match.status === 'skipped') {
      queue[i] = {
        schema: entry.schema,
        table: entry.table,
        status: 'skipped',
        message: match.message ?? messages.skipExistingMessage,
      }
      skipped++
    } else {
      queue[i] = {
        schema: entry.schema,
        table: entry.table,
        status: 'error',
        message: match.message ?? messages.networkErrorMessage,
      }
      errCount++
    }
  }
  return { ok, error: errCount, skipped }
}

export async function runSequentialBulkDescribe(opts: {
  targets: BulkDescribeTarget[]
  queue: BulkEntry[]
  datasourceId: string
  sampleSize: number
  locale?: string
  networkErrorMessage: string
  okColumnsMessage: (cols: number) => string
  isCancelled: () => boolean
  setBulkEntries: (entries: BulkEntry[]) => void
  runOne: <TReq extends object, TRes>(
    kind: 'describe',
    request: TReq,
  ) => Promise<TRes | 'fallback' | null>
}): Promise<BulkDescribeSummary> {
  let ok = 0
  let errCount = 0
  const skipped = opts.queue.filter((q) => q.status === 'skipped').length

  for (let i = 0; i < opts.targets.length; i++) {
    if (opts.isCancelled()) {
      break
    }
    const row = opts.targets[i]
    const entry = opts.queue[i]
    if (!row || !entry || entry.status === 'skipped') {
      continue
    }

    const schema = row.schema_name
    const table = row.table_name
    opts.queue[i] = { schema, table, status: 'running' }
    opts.setBulkEntries([...opts.queue])

    try {
      const request: DescribeJobRequest = {
        datasource_id: opts.datasourceId,
        schema,
        table,
        locale: opts.locale,
        sample_size: opts.sampleSize,
        auto_apply: true,
      }
      let data = await opts.runOne<typeof request, DescribeResult>('describe', request)
      if (data === 'fallback') {
        data = await runMetadataDescribeDirect(request)
      }
      if (!data) {
        opts.queue[i] = { schema, table, status: 'error', message: opts.networkErrorMessage }
        errCount++
      } else {
        const cols = data.columns.length
        opts.queue[i] = {
          schema,
          table,
          status: 'ok',
          message: opts.okColumnsMessage(cols),
        }
        ok++
      }
    } catch (err) {
      opts.queue[i] = {
        schema,
        table,
        status: 'error',
        message: err instanceof Error ? err.message : opts.networkErrorMessage,
      }
      errCount++
    }
    opts.setBulkEntries([...opts.queue])
  }

  return { ok, error: errCount, skipped }
}

export async function runBulkDescribeEnqueue(opts: {
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
}): Promise<void> {
  const batchRequest = buildDescribeBatchRequest({
    datasourceId: opts.datasourceId,
    targets: opts.targets,
    sampleSize: opts.sampleSize,
    locale: opts.locale,
    skipExisting: opts.skipExisting,
  })

  const {
    data: enqueued,
    status,
    error,
  } = await fetchJSON<AIJob | DescribeBatchConflictBody>('/api/ai/jobs', {
    method: 'POST',
    body: JSON.stringify({
      client_session_id: opts.sessionId,
      kind: 'describe_batch',
      request: batchRequest,
    }),
  })
  if (opts.isStale()) {
    return
  }

  if (error) {
    failBulkEnqueue(opts, error)
    return
  }

  if (status === 409 && isDescribeBatchConflictBody(enqueued)) {
    handleBulkDescribeConflict(opts, enqueued)
    return
  }

  if (status >= 200 && status < 300 && isAIJob(enqueued)) {
    await processEnqueuedBatchJob(opts, enqueued, batchRequest)
  } else {
    await finishBulkDescribeWithoutJob(opts)
  }

  if (!opts.isStale()) {
    opts.setBulkRunning(false)
  }
  opts.onFinished?.()
}

export function buildDescribeBatchRequest(opts: {
  datasourceId: string
  targets: BulkDescribeTarget[]
  sampleSize: number
  locale?: string
  skipExisting: boolean
}): DescribeBatchJobRequest {
  return {
    datasource_id: opts.datasourceId,
    tables: opts.targets.map((row) => ({
      schema: row.schema_name,
      table: row.table_name,
    })),
    sample_size: opts.sampleSize,
    locale: opts.locale,
    auto_apply: true,
    skip_existing: opts.skipExisting,
  }
}
