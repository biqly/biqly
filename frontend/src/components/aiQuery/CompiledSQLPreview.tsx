import { useId, useState } from 'react'

import { compileQuery, dryRunQuery } from '../../api/query'
import { useT } from '../../i18n'
import { buttonClass, rowActionsClass } from '../../lib/buttonClasses'
import { errorAlertClass, sqlPreviewClass } from '../../lib/feedbackClasses'
import type { LogicalQuery } from '../../types/ai'
import { tokenizeSQL } from './compiledSQLTokens'

function SQLCode({ sql, ariaLabel }: { sql: string; ariaLabel: string }) {
  return (
    <pre className={sqlPreviewClass} aria-label={ariaLabel}>
      <code>
        {tokenizeSQL(sql).map((token, index) => (
          <span
            key={`${token.value}-${index}`}
            className={token.kind === 'keyword' ? 'text-accent font-semibold' : undefined}
          >
            {token.value}
          </span>
        ))}
      </code>
    </pre>
  )
}

export function CompiledSQLPreview({
  logicalQuery,
  initialSQL,
}: {
  logicalQuery?: LogicalQuery
  initialSQL?: string
}) {
  const t = useT()
  const previewID = useId()
  const [expanded, setExpanded] = useState(false)
  const [sql, setSQL] = useState(initialSQL ?? '')
  const [loading, setLoading] = useState(false)
  const [validating, setValidating] = useState(false)
  const [validation, setValidation] = useState<'valid' | null>(null)
  const [error, setError] = useState<string | null>(null)

  const compile = async () => {
    if (!logicalQuery || loading) {
      return
    }
    setLoading(true)
    setError(null)
    try {
      const response = await compileQuery(logicalQuery)
      setSQL(response.sql)
    } catch {
      setError(t('ai_query.sql_preview_compile_failed'))
    } finally {
      setLoading(false)
    }
  }

  const toggle = () => {
    const next = !expanded
    setExpanded(next)
    if (next && !sql) {
      void compile()
    }
  }

  const validate = async () => {
    if (!logicalQuery || validating) {
      return
    }
    setValidating(true)
    setValidation(null)
    setError(null)
    try {
      const response = await dryRunQuery(logicalQuery)
      setSQL(response.sql)
      setValidation('valid')
    } catch {
      setError(t('ai_query.sql_preview_validate_failed'))
    } finally {
      setValidating(false)
    }
  }

  return (
    <section className="mt-3">
      <button
        type="button"
        className="text-foreground flex w-full items-center justify-between gap-2 border-0 bg-transparent p-0 text-left text-sm font-semibold"
        aria-expanded={expanded}
        aria-controls={previewID}
        onClick={toggle}
      >
        {t('ai_query.sql_preview_title')}
        <span aria-hidden="true" className="text-foreground-faint text-[0.65rem]">
          {expanded ? '▲' : '▼'}
        </span>
      </button>
      {expanded ? (
        <div id={previewID} className="mt-2" aria-busy={loading || validating}>
          {loading ? (
            <p className="text-foreground-faint m-0 text-sm">{t('ai_query.sql_preview_loading')}</p>
          ) : null}
          {sql ? <SQLCode sql={sql} ariaLabel={t('ai_query.sql_preview_code_aria')} /> : null}
          {logicalQuery && sql ? (
            <div className={rowActionsClass}>
              <button
                type="button"
                className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
                disabled={validating}
                onClick={() => void validate()}
              >
                {validating
                  ? t('ai_query.sql_preview_validating')
                  : t('ai_query.sql_preview_validate')}
              </button>
            </div>
          ) : null}
          {validation === 'valid' ? (
            <p className="text-success m-2 text-sm" role="status">
              {t('ai_query.sql_preview_valid')}
            </p>
          ) : null}
          {error ? (
            <p className={errorAlertClass} role="alert">
              {error}
            </p>
          ) : null}
        </div>
      ) : null}
    </section>
  )
}
