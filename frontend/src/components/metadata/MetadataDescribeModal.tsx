import { useEffect, useRef, useState } from 'react'

import { type DescribeResult, runMetadataDescribeDirect } from '../../api/metadataDescribe'
import type { DescribeJobRequest } from '../../hooks/useAIJobsUtils'
import type { Locale } from '../../i18n'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { modalActionsClass } from '../../lib/modalClasses'
import type { AIRuntimeSettings } from '../../types/ai'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'
import { MetadataDescribeForm } from './MetadataDescribeForm'
import { MetadataDescribeResults } from './MetadataDescribeResults'

type DescribePhase = 'setup' | 'running' | 'results'

function describeTranslationModel(
  result: DescribeResult | null,
  aiRuntime: AIRuntimeSettings | null,
): string | undefined {
  if (result?.translation_applied) {
    return result.translation_model
  }
  return aiRuntime?.translation_enabled ? aiRuntime.translation_model : undefined
}

function DescribeRunningView({
  t,
  error,
  onBackground,
}: {
  t: ReturnType<typeof useT>
  error: string | null
  onBackground: () => void
}) {
  return (
    <>
      <div className="flex flex-col items-center justify-center gap-3 py-6">
        <div className="border-border-strong border-t-accent h-8 w-8 animate-spin rounded-full border-2" />
        <p className="text-foreground-muted m-0 text-[0.82rem]">
          {t('metadata.describe_analyzing')}
        </p>
        {error && (
          <p className="text-error m-0 text-[0.78rem]" role="alert">
            {error}
          </p>
        )}
      </div>
      <div className={cn(modalActionsClass(), 'border-border mt-0 border-t pt-[0.85rem]')}>
        <button
          type="button"
          className={buttonClass('ghost', { size: 'sm' })}
          onClick={onBackground}
        >
          {t('metadata.bulk_run_background')}
        </button>
      </div>
    </>
  )
}

interface MetadataDescribeModalProps {
  open: boolean
  table: TableRow
  datasourceId: string
  columns: ColumnRow[]
  aiRuntime: AIRuntimeSettings | null
  apiError: string | null
  runDescribeJob: (
    request: DescribeJobRequest,
    onError: (message: string) => void,
  ) => Promise<DescribeResult | 'fallback' | null>
  patchDescription: (kind: 'table' | 'column', id: string, description: string) => Promise<void>
  onClose: () => void
  onApplied: (table: TableRow) => void
  locale: Locale
}

export function MetadataDescribeModal({
  open,
  table,
  datasourceId,
  columns,
  aiRuntime,
  apiError,
  runDescribeJob,
  patchDescription,
  onClose,
  onApplied,
  locale,
}: MetadataDescribeModalProps) {
  const t = useT()
  const [form, setForm] = useState({ sample_size: 10, auto_apply: true })
  const [result, setResult] = useState<DescribeResult | null>(null)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [phase, setPhase] = useState<DescribePhase>('setup')
  const modalOpenRef = useRef(open)
  useEffect(() => {
    modalOpenRef.current = open
  }, [open])
  const fqn = `${table.schema_name}.${table.table_name}`

  const dbManaged = aiRuntime?.db_managed === true
  const managedRuntime = dbManaged ? aiRuntime : null
  const activeDescribe = managedRuntime?.active_models?.find((m) => m.purpose === 'describe')
  const activeTranslation = managedRuntime?.active_models?.find((m) => m.purpose === 'translation')

  const closeWithReset = () => {
    setPhase('setup')
    setResult(null)
    setError(null)
    setRunning(false)
    onClose()
  }

  const runDescribe = async () => {
    setRunning(true)
    setError(null)
    setPhase('running')
    const request: DescribeJobRequest = {
      datasource_id: datasourceId,
      schema: table.schema_name,
      table: table.table_name,
      locale,
      sample_size: form.sample_size,
      auto_apply: form.auto_apply,
    }
    try {
      let res = await runDescribeJob(request, (message) => setError(message))
      if (res === 'fallback') {
        res = await runMetadataDescribeDirect(request)
      }
      if (!modalOpenRef.current) {
        return
      }
      if (res) {
        setResult(res)
        setPhase('results')
        if (res.applied) {
          onApplied(table)
        }
      } else {
        setPhase('setup')
      }
    } catch (err) {
      if (!modalOpenRef.current) {
        return
      }
      setError(err instanceof Error ? err.message : t('metadata.bulk_network_error'))
      setPhase('setup')
    } finally {
      setRunning(false)
    }
  }

  return (
    <Modal
      open={open}
      title={
        <div>
          <h2
            id="describe-title"
            className="m-0 text-[0.95rem] leading-tight font-[650] tracking-[-0.02em]"
          >
            {t('metadata.btn_ai_describe')}
          </h2>
          <p className="text-foreground-faint mx-0 mt-[0.2rem] mb-0 max-w-lg text-[0.72rem] leading-[1.35]">
            <span translate="no">{fqn}</span>
          </p>
        </div>
      }
      subtitle={
        <ModelBadgeRow
          primaryLabel={t('metadata.describe_badge_label')}
          primaryModel={result?.model ?? aiRuntime?.llm_model}
          primaryNote={dbManaged ? activeDescribe?.provider_name : undefined}
          translationModel={describeTranslationModel(result, aiRuntime)}
          translationNote={dbManaged ? activeTranslation?.provider_name : undefined}
        />
      }
      labelledBy="describe-title"
      onClose={closeWithReset}
      className={cn(
        'border-border bg-card flex max-h-[min(calc(100vh-1.5rem),90vh)] min-h-0 w-[min(100%,40rem)] flex-col overflow-hidden rounded-(--radius) border shadow-(--shadow)',
        phase === 'results' && 'w-[min(100%,52rem)]',
      )}
      bodyClassName={
        phase === 'results'
          ? 'gap-4 overflow-y-auto min-h-0 flex-1 p-[0.85rem_1rem_1rem]'
          : 'gap-[0.65rem] p-[0.85rem_1rem_1rem]'
      }
    >
      {phase === 'setup' && (
        <MetadataDescribeForm
          t={t}
          sampleSize={form.sample_size}
          autoApply={form.auto_apply}
          running={running}
          error={error}
          apiError={apiError}
          onSampleSizeChange={(size) => setForm({ ...form, sample_size: size })}
          onAutoApplyChange={(checked) => setForm({ ...form, auto_apply: checked })}
          onClose={closeWithReset}
          onRun={() => void runDescribe()}
        />
      )}

      {phase === 'running' && (
        <DescribeRunningView t={t} error={error} onBackground={closeWithReset} />
      )}

      {phase === 'results' && result && (
        <>
          <MetadataDescribeResults
            result={result}
            t={t}
            onApplyTable={(description) => {
              void patchDescription('table', table.id, description)
            }}
            onApplyColumn={(name, description) => {
              const col = columns.find((c) => c.column_name === name)
              if (col) {
                void patchDescription('column', col.id, description)
              }
            }}
          />
          <footer
            className={cn(
              modalActionsClass(),
              'border-border mt-0 shrink-0 border-t px-0 pt-[0.85rem]',
            )}
          >
            <button
              type="button"
              className={buttonClass('ghost', { size: 'sm' })}
              onClick={closeWithReset}
            >
              {t('metadata.describe_close_footer')}
            </button>
          </footer>
        </>
      )}
    </Modal>
  )
}
