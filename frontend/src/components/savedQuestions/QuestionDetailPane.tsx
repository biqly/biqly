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
  t: any
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
        <p className="saved-question-description" style={{ marginTop: '0.5rem' }}>
          {selectedQuestion.description}
        </p>
      )}

      <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', margin: '1rem 0' }}>
        {selectedQuestion.model_id && (
          <span
            className="tag-pill"
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}
          >
            <strong>{t('saved_questions.label_select_model')}:</strong>
            <code>
              {semanticModels.find((m) => m.id === selectedQuestion.model_id)?.label ||
                selectedQuestion.model_id}
            </code>
          </span>
        )}
        <span
          className="tag-pill"
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}
        >
          <strong>{t('saved_questions.label_dialect')}:</strong>
          <code>{selectedQuestion.dialect}</code>
        </span>
        {selectedQuestion.locale && (
          <span
            className="tag-pill"
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}
          >
            <strong>{t('saved_questions.label_locale')}:</strong>
            <code>{selectedQuestion.locale}</code>
          </span>
        )}
      </div>

      <h3 style={{ marginTop: '1.5rem', marginBottom: '0.5rem' }}>
        {t('saved_questions.label_question')}
      </h3>
      <p
        style={{
          background: 'var(--bg-card-raised)',
          padding: '0.75rem 1rem',
          borderRadius: '0.35rem',
          fontStyle: 'italic',
        }}
      >
        {selectedQuestion.question}
      </p>

      <h3 style={{ marginTop: '1.5rem', marginBottom: '0.5rem' }}>
        {t('saved_questions.logical_query_heading')}
      </h3>
      <pre className="sql-preview" style={{ maxHeight: '250px', overflowY: 'auto' }}>
        {JSON.stringify(selectedQuestion.logical_query, null, 2)}
      </pre>

      <div className="saved-question-actions">
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => onRun(selectedQuestion.logical_query)}
          disabled={runLoading}
          aria-label={t('saved_questions.aria_run_query')}
        >
          {t('saved_questions.run_query')}
        </button>
        <button
          type="button"
          className="btn"
          onClick={() => onOpenEdit(selectedQuestion)}
          aria-label={t('saved_questions.aria_edit_query')}
        >
          {t('saved_questions.edit_query')}
        </button>
        <button
          type="button"
          className="btn btn-danger"
          onClick={() => onDelete(selectedQuestion.id)}
          aria-label={t('saved_questions.aria_delete_query')}
        >
          {t('saved_questions.delete_query')}
        </button>
      </div>

      {/* Inline query execution results */}
      {runLoading && (
        <div
          style={{
            marginTop: '1.5rem',
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: 100,
          }}
        >
          <LoadingOverlay loading={true} />
        </div>
      )}

      {runError && (
        <div style={{ marginTop: '1.5rem' }}>
          <ErrorAlert error={runError} />
        </div>
      )}

      {runResult?.columns && runResult.rows && (
        <div className="results-section" style={{ marginTop: '1.5rem' }}>
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
