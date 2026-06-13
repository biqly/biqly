import { useEffect, useState } from 'react'

import { type DescribeResult, runMetadataDescribeDirect } from '../../api/metadataDescribe'
import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import {
  modalActionsBorderedClass,
  modalBackdropClass,
  modalCloseClass,
  modalDescribeBodyClass,
  modalDescribeCardClass,
  modalDescribeFqnClass,
  modalDescribeHeaderClass,
  modalDescribeIntroClass,
  modalDescribeTitleClass,
} from '../../lib/modalClasses'
import type { AIRuntimeSettings } from '../../types/ai'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'
import { MetadataDescribeForm } from './MetadataDescribeForm'
import { MetadataDescribeResults } from './MetadataDescribeResults'

interface MetadataDescribeModalProps {
  table: TableRow
  datasourceId: string
  columns: ColumnRow[]
  aiRuntime: AIRuntimeSettings | null
  apiError: string | null
  runDescribeJob: (
    request: {
      datasource_id: string
      schema: string
      table: string
      sample_size: number
      auto_apply: boolean
    },
    onError: (message: string) => void,
  ) => Promise<DescribeResult | 'fallback' | null>
  patchDescription: (kind: 'table' | 'column', id: string, description: string) => Promise<void>
  onClose: () => void
  onApplied: (table: TableRow) => void
}

function DescribeModalHeader({
  fqn,
  result,
  aiRuntime,
  onClose,
}: {
  fqn: string
  result: DescribeResult | null
  aiRuntime: AIRuntimeSettings | null
  onClose: () => void
}) {
  const t = useT()
  const dbManaged = aiRuntime?.db_managed === true
  const managedRuntime = dbManaged ? aiRuntime : null
  const activeDescribe = managedRuntime?.active_models?.find((m) => m.purpose === 'describe')
  const activeTranslation = managedRuntime?.active_models?.find((m) => m.purpose === 'translation')
  const translationModel = result?.translation_applied
    ? result.translation_model
    : aiRuntime?.translation_enabled
      ? aiRuntime.translation_model
      : undefined

  return (
    <header className={modalDescribeHeaderClass}>
      <div className="min-w-0 flex-1">
        <h2 id="describe-title" className={modalDescribeTitleClass}>
          {t('metadata.btn_ai_describe')}
        </h2>
        <p className={modalDescribeFqnClass} translate="no">
          {fqn}
        </p>
        <ModelBadgeRow
          className="mt-1.5"
          primaryLabel={t('metadata.describe_badge_label')}
          primaryModel={result?.model ?? aiRuntime?.llm_model}
          primaryNote={dbManaged ? activeDescribe?.provider_name : undefined}
          translationModel={translationModel}
          translationNote={dbManaged ? activeTranslation?.provider_name : undefined}
        />
      </div>
      <button
        type="button"
        className={cn(modalCloseClass(), 'mt-0.5 shrink-0')}
        aria-label={t('metadata.describe_close_aria')}
        onClick={onClose}
      >
        ×
      </button>
    </header>
  )
}

export function MetadataDescribeModal({
  table,
  datasourceId,
  columns,
  aiRuntime,
  apiError,
  runDescribeJob,
  patchDescription,
  onClose,
  onApplied,
}: MetadataDescribeModalProps) {
  const t = useT()
  const [form, setForm] = useState({ sample_size: 10, auto_apply: false })
  const [result, setResult] = useState<DescribeResult | null>(null)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const fqn = `${table.schema_name}.${table.table_name}`

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  const runDescribe = async () => {
    setRunning(true)
    setError(null)
    const request = {
      datasource_id: datasourceId,
      schema: table.schema_name,
      table: table.table_name,
      sample_size: form.sample_size,
      auto_apply: form.auto_apply,
    }
    try {
      let res = await runDescribeJob(request, (message) => setError(message))
      if (res === 'fallback') {
        res = await runMetadataDescribeDirect(request)
      }
      if (res) {
        setResult(res)
        if (res.applied) {
          onApplied(table)
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('metadata.bulk_network_error'))
    } finally {
      setRunning(false)
    }
  }

  const applySuggestion = async (kind: 'table' | 'column', name: string, description: string) => {
    if (kind === 'table') {
      await patchDescription('table', table.id, description)
    } else {
      const col = columns.find((c) => c.column_name === name)
      if (!col) {
        return
      }
      await patchDescription('column', col.id, description)
    }
  }

  return (
    <div
      className={modalBackdropClass()}
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose()
        }
      }}
    >
      <section
        className={modalDescribeCardClass(!!result)}
        role="dialog"
        aria-modal="true"
        aria-labelledby="describe-title"
      >
        <DescribeModalHeader fqn={fqn} result={result} aiRuntime={aiRuntime} onClose={onClose} />

        <div className={modalDescribeBodyClass(!!result)}>
          {!result && (
            <>
              <p className={modalDescribeIntroClass}>{t('metadata.describe_intro')}</p>
              <MetadataDescribeForm
                t={t}
                sampleSize={form.sample_size}
                autoApply={form.auto_apply}
                running={running}
                error={error}
                apiError={apiError}
                onSampleSizeChange={(size) => setForm({ ...form, sample_size: size })}
                onAutoApplyChange={(checked) => setForm({ ...form, auto_apply: checked })}
                onClose={onClose}
                onRun={() => void runDescribe()}
              />
            </>
          )}

          {result && (
            <MetadataDescribeResults
              result={result}
              t={t}
              onApplyTable={(description) => void applySuggestion('table', '', description)}
              onApplyColumn={(name, description) =>
                void applySuggestion('column', name, description)
              }
            />
          )}
        </div>

        {result && (
          <footer className={cn(modalActionsBorderedClass(), 'shrink-0 px-5 pb-4')}>
            <button
              type="button"
              className={legacyButtonClass('btn btn-ghost btn-sm')}
              onClick={() => {
                setResult(null)
                onClose()
              }}
            >
              {t('metadata.describe_close_footer')}
            </button>
          </footer>
        )}
      </section>
    </div>
  )
}
