import { useT } from '../../i18n'
import type { GenerationTrace } from '../../types/ai'
import { Collapsible } from './routingViz'

function formatConfidence(confidence: number): string {
  if (!Number.isFinite(confidence) || confidence <= 0) {
    return ''
  }
  return `${Math.round(confidence * 100)}%`
}

export function GenerationTracePanel({
  trace,
  defaultOpen = false,
}: {
  trace: GenerationTrace
  defaultOpen?: boolean
}) {
  const t = useT()
  const routeConfidence =
    trace.route_confidence != null ? formatConfidence(trace.route_confidence) : ''

  return (
    <Collapsible title={t('ai_query.generation_trace_title')} defaultOpen={defaultOpen}>
      <div className="generation-trace">
        {trace.routed_table ? (
          <p className="generation-trace__row">
            <span className="generation-trace__label">{t('ai_query.generation_trace_table')}</span>
            <code>{trace.routed_table}</code>
            {routeConfidence ? (
              <span className="generation-trace__meta">
                {t('ai_query.generation_trace_confidence', { value: routeConfidence })}
              </span>
            ) : null}
          </p>
        ) : null}

        {trace.ambiguity_result ? (
          <p className="generation-trace__row">
            <span className="generation-trace__label">
              {t('ai_query.generation_trace_ambiguity')}
            </span>
            <span>{trace.ambiguity_result}</span>
          </p>
        ) : null}

        {trace.ambiguity_detail ? (
          <p className="generation-trace__detail">
            <span className="generation-trace__label">
              {t('ai_query.generation_trace_ambiguity_detail')}
            </span>
            {trace.ambiguity_detail}
          </p>
        ) : null}

        {(trace.columns_resolved?.length ?? 0) > 0 ? (
          <>
            <p className="generation-trace__row">
              <span className="generation-trace__label">
                {t('ai_query.generation_trace_columns')}
              </span>
            </p>
            <ul className="generation-trace__columns">
              {(trace.columns_resolved ?? []).map((col, idx) => (
                <li key={`${col.term}-${col.resolved}-${idx}`}>
                  <code>{col.term}</code>
                  <span className="generation-trace__arrow" aria-hidden="true">
                    →
                  </span>
                  <code>{col.resolved}</code>
                  {col.source ? (
                    <span className="generation-trace__meta">({col.source})</span>
                  ) : null}
                </li>
              ))}
            </ul>
          </>
        ) : null}
      </div>
    </Collapsible>
  )
}
