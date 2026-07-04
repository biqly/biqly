import type { useT } from '../../i18n'
import { EmptyState } from '../ui/EmptyState'
import { TagBadge } from '../ui/TagBadge'
import type { KnowledgeItem } from './types'

function DetailRow({ label, value }: { label: string; value: string }) {
  if (!value) {
    return null
  }
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-muted-foreground text-xs font-medium tracking-wide uppercase">{label}</dt>
      <dd className="m-0 text-sm">{value}</dd>
    </div>
  )
}

function SynonymList({ label, synonyms }: { label: string; synonyms: string[] }) {
  if (synonyms.length === 0) {
    return null
  }
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-muted-foreground text-xs font-medium tracking-wide uppercase">{label}</dt>
      <dd className="m-0 flex flex-wrap gap-1">
        {synonyms.map((s) => (
          <TagBadge key={s}>{s}</TagBadge>
        ))}
      </dd>
    </div>
  )
}

function TermDetail({ item, t }: { item: KnowledgeItem; t: ReturnType<typeof useT> }) {
  const term = item.term
  if (!term) {
    return null
  }
  return (
    <dl className="m-0 flex flex-col gap-3">
      <DetailRow label={t('knowledge.field_definition')} value={term.definition ?? ''} />
      <DetailRow
        label={t('knowledge.field_maps_to')}
        value={`${term.maps_to_type}: ${term.maps_to_name}`}
      />
      <SynonymList label={t('knowledge.field_synonyms')} synonyms={item.synonyms} />
      <DetailRow label={t('knowledge.field_unit')} value={term.ai_context?.unit ?? ''} />
      <DetailRow
        label={t('knowledge.field_business_rules')}
        value={(term.ai_context?.business_rules ?? []).join('; ')}
      />
    </dl>
  )
}

function MetricDetail({ item, t }: { item: KnowledgeItem; t: ReturnType<typeof useT> }) {
  const metric = item.metric
  if (!metric) {
    return null
  }
  return (
    <dl className="m-0 flex flex-col gap-3">
      <DetailRow label={t('knowledge.field_description')} value={metric.description ?? ''} />
      <DetailRow
        label={t('knowledge.field_expression')}
        value={`${metric.aggregation}(${metric.expression})`}
      />
      <DetailRow label={t('knowledge.field_format')} value={metric.format ?? ''} />
      <DetailRow label={t('knowledge.field_model')} value={item.modelName} />
      <SynonymList label={t('knowledge.field_synonyms')} synonyms={item.synonyms} />
    </dl>
  )
}

function DimensionDetail({ item, t }: { item: KnowledgeItem; t: ReturnType<typeof useT> }) {
  const dimension = item.dimension
  if (!dimension) {
    return null
  }
  const enums = dimension.enum_values ?? []
  return (
    <dl className="m-0 flex flex-col gap-3">
      <DetailRow label={t('knowledge.field_description')} value={dimension.description ?? ''} />
      <DetailRow
        label={t('knowledge.field_column')}
        value={dimension.calculated_expression ?? dimension.column_ref}
      />
      <DetailRow label={t('knowledge.field_type')} value={dimension.type} />
      <DetailRow label={t('knowledge.field_model')} value={item.modelName} />
      <SynonymList label={t('knowledge.field_synonyms')} synonyms={item.synonyms} />
      {enums.length > 0 && (
        <div className="flex flex-col gap-1">
          <dt className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
            {t('knowledge.field_enum_values')}
          </dt>
          <dd className="m-0 flex flex-wrap gap-1">
            {enums.map((ev) => (
              <TagBadge key={ev.raw_value}>{`${ev.raw_value} → ${ev.label}`}</TagBadge>
            ))}
          </dd>
        </div>
      )}
    </dl>
  )
}

function AnswerDetail({ item, t }: { item: KnowledgeItem; t: ReturnType<typeof useT> }) {
  const answer = item.answer
  if (!answer) {
    return null
  }
  return (
    <dl className="m-0 flex flex-col gap-3">
      <DetailRow label={t('knowledge.field_question')} value={answer.question} />
      <DetailRow label={t('knowledge.field_description')} value={item.subtitle} />
      <SynonymList label={t('knowledge.field_tags')} synonyms={answer.tags} />
      <div className="flex flex-col gap-1">
        <dt className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
          {t('knowledge.field_logical_query')}
        </dt>
        <dd className="m-0">
          <pre className="bg-muted/30 border-border max-h-72 overflow-auto rounded border p-3 text-xs leading-5">
            {JSON.stringify(answer.logical_query, null, 2)}
          </pre>
        </dd>
      </div>
    </dl>
  )
}

export function KnowledgeDetail({
  item,
  t,
}: {
  item: KnowledgeItem | null
  t: ReturnType<typeof useT>
}) {
  if (!item) {
    return <EmptyState description={t('knowledge.select_item')} />
  }
  return (
    <div className="flex flex-col gap-4">
      <div>
        <h3 className="m-0 text-base font-semibold">{item.title}</h3>
        {item.modelName && <p className="text-muted-foreground m-0 text-xs">{item.modelName}</p>}
      </div>
      {item.category === 'terms' && <TermDetail item={item} t={t} />}
      {item.category === 'metrics' && <MetricDetail item={item} t={t} />}
      {item.category === 'dimensions' && <DimensionDetail item={item} t={t} />}
      {item.category === 'answers' && <AnswerDetail item={item} t={t} />}
    </div>
  )
}
