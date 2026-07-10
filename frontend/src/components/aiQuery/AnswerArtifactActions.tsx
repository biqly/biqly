import { useNavigate } from 'react-router-dom'

import type { TFunction } from '../../i18n'
import type { AIQueryResponse } from '../../types/ai'

interface AnswerArtifactActionsProps {
  result: AIQueryResponse
  datasourceId: string
  userQuestion: string
  t: TFunction
}

/**
 * Post-answer artifact actions (spec §15). A successful analysis produces a
 * reusable LogicalQuery; the primary artifact is a saved query, created by
 * handing the analysis off to the Saved Questions form via its existing
 * `?prefill=1` route (no new API). Only shown when the answer carries a
 * LogicalQuery — a text-only or failed answer has nothing to save.
 */
export function AnswerArtifactActions({
  result,
  datasourceId,
  userQuestion,
  t,
}: AnswerArtifactActionsProps) {
  const navigate = useNavigate()
  const logicalQuery = result.logical_query
  if (!logicalQuery) {
    return null
  }

  const saveAsQuery = () => {
    const params = new URLSearchParams({
      prefill: '1',
      datasource_id: logicalQuery.datasource_id || datasourceId,
      question: userQuestion,
      logical_query: JSON.stringify(logicalQuery),
    })
    void navigate(`/saved?${params.toString()}`)
  }

  return (
    <section
      className="border-border/70 mt-1 flex flex-wrap items-center gap-2 border-t pt-2.5"
      aria-label={t('ai_query.artifacts_title')}
    >
      <p className="text-foreground-muted m-0 text-[0.78rem] font-medium">
        {t('ai_query.artifacts_title')}
      </p>
      <button
        type="button"
        className="border-border bg-card-raised text-foreground hover:border-accent hover:text-accent focus-visible:ring-accent inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-[0.78rem] transition focus-visible:ring-2 focus-visible:outline-none"
        onClick={saveAsQuery}
      >
        <svg width="13" height="13" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M5 3h11l3 3v15a0 0 0 0 1 0 0H5a0 0 0 0 1 0 0V3Z M8 3v5h7 M8 21v-6h8v6"
          />
        </svg>
        {t('ai_query.artifacts_save_query')}
      </button>
    </section>
  )
}
