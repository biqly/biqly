import type { TFunction } from '../../i18n'
import { tagPillMetaClass } from '../../lib/badgeClasses'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import {
  savedQuestionActionsClass,
  savedQuestionDescriptionClass,
} from '../../lib/savedQuestionClasses'
import { ResultTable } from '../ResultTable'
import { EmptyState } from '../ui/EmptyState'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import type { SavedQuestion, SavedQuestionSemanticModel } from './types'
interface QuestionDetailPaneProps {
  selectedQuestion: SavedQuestion | null
  semanticModels: SavedQuestionSemanticModel[]
  runLoading: boolean
  runError: string | null
  runResult: {
    columns: { name: string; type?: string }[]
    rows: unknown[][]
    stats?: { duration_ms?: number }
  } | null
  onRun: (logicalQuery: Record<string, unknown>) => void
  onOpenEdit: (q: SavedQuestion) => void
  onDelete: (id: string) => void
  t: TFunction
}

export function QuestionDetailPane({
  selectedQuestion,
  semanticModels,
  runLoading,
  runError,
  runResult,
  onRun,
  onOpenEdit,
  onDelete,
  t,
}: QuestionDetailPaneProps) {
  if (!selectedQuestion) {
    return <EmptyState description={t('saved_questions.select_hint')} />
  }

  return (
    <div>
      <h2>{selectedQuestion.name}</h2>
      {selectedQuestion.description && (
        <p className={`${savedQuestionDescriptionClass()} mt-2`}>{selectedQuestion.description}</p>
      )}

      <div className="my-4 flex flex-wrap gap-2">
        {selectedQuestion.model_id && (
          <span className={tagPillMetaClass}>
            <strong>{t('saved_questions.label_select_model')}:</strong>
            <code>
              {semanticModels.find((m) => m.id === selectedQuestion.model_id)?.label ??
                selectedQuestion.model_id}
            </code>
          </span>
        )}
        <span className={tagPillMetaClass}>
          <strong>{t('saved_questions.label_dialect')}:</strong>
          <code>{selectedQuestion.dialect}</code>
        </span>
        {selectedQuestion.locale && (
          <span className={tagPillMetaClass}>
            <strong>{t('saved_questions.label_locale')}:</strong>
            <code>{selectedQuestion.locale}</code>
          </span>
        )}
      </div>

      <h3 className="mt-6 mb-2">{t('saved_questions.label_question')}</h3>
      <p className="bg-card-raised rounded-md px-4 py-3 italic">{selectedQuestion.question}</p>

      <h3 className="mt-6 mb-2">{t('saved_questions.logical_query_heading')}</h3>
      <pre className={cn(legacyFeedbackClass('sql-preview'), 'max-h-[250px] overflow-y-auto')}>
        {JSON.stringify(selectedQuestion.logical_query, null, 2)}
      </pre>

      <div className={savedQuestionActionsClass()}>
        <button
          type="button"
          className={legacyButtonClass('btn btn-primary')}
          onClick={() => onRun(selectedQuestion.logical_query)}
          disabled={runLoading}
          aria-label={t('saved_questions.aria_run_query')}
        >
          {t('saved_questions.run_query')}
        </button>
        <button
          type="button"
          className={legacyButtonClass('btn')}
          onClick={() => onOpenEdit(selectedQuestion)}
          aria-label={t('saved_questions.aria_edit_query')}
        >
          {t('saved_questions.edit_query')}
        </button>
        <button
          type="button"
          className={legacyButtonClass('btn btn-danger')}
          onClick={() => onDelete(selectedQuestion.id)}
          aria-label={t('saved_questions.aria_delete_query')}
        >
          {t('saved_questions.delete_query')}
        </button>
      </div>

      {/* Inline query execution results */}
      {runLoading && (
        <div className="mt-6 flex min-h-25 items-center justify-center">
          <LoadingOverlay loading={true} />
        </div>
      )}

      {runError && (
        <div className="mt-6">
          <ErrorAlert error={runError} />
        </div>
      )}

      {runResult && (
        <div className="results-section mt-6">
          <ResultTable
            columns={runResult.columns}
            rows={runResult.rows}
            rowCount={runResult.rows.length}
            durationMs={runResult.stats?.duration_ms}
          />
        </div>
      )}
    </div>
  )
}
