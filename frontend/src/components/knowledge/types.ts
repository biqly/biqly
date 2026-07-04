import type { BusinessGlossaryTerm } from '../../types/glossary'
import type { SemanticDimension, SemanticMetric, SemanticModelDetail } from '../../types/semantic'

export type KnowledgeCategory = 'terms' | 'metrics' | 'dimensions' | 'answers'

export const KNOWLEDGE_CATEGORIES: KnowledgeCategory[] = [
  'terms',
  'metrics',
  'dimensions',
  'answers',
]

export interface VerifiedAnswer {
  id: string
  datasource_id: string
  model_id?: string
  question: string
  logical_query: Record<string, unknown>
  tags: string[]
  name?: string
  description?: string
  updated_at?: string
}

export interface KnowledgeItem {
  id: string
  category: KnowledgeCategory
  title: string
  subtitle: string
  modelName: string
  synonyms: string[]
  searchText: string
  term?: BusinessGlossaryTerm
  metric?: SemanticMetric
  dimension?: SemanticDimension
  answer?: VerifiedAnswer
}

function searchable(parts: (string | null | undefined)[]): string {
  return parts
    .filter((p): p is string => !!p)
    .join(' ')
    .toLocaleLowerCase()
}

export function buildTermItems(terms: BusinessGlossaryTerm[]): KnowledgeItem[] {
  return terms
    .filter((term) => term.is_active)
    .map((term) => {
      const synonyms = [...(term.aliases ?? []), ...(term.ai_context?.synonyms ?? [])]
      return {
        id: `term-${term.id}`,
        category: 'terms' as const,
        title: term.term,
        subtitle: term.definition ?? '',
        modelName: '',
        synonyms,
        searchText: searchable([
          term.term,
          term.definition,
          term.maps_to_name,
          term.maps_to_type,
          ...synonyms,
        ]),
        term,
      }
    })
}

export function buildModelItems(models: SemanticModelDetail[]): KnowledgeItem[] {
  const items: KnowledgeItem[] = []
  for (const model of models) {
    const modelName = model.label ?? model.name
    for (const metric of model.metrics ?? []) {
      if (metric.is_active === false) {
        continue
      }
      items.push({
        id: `metric-${model.id}-${metric.id}`,
        category: 'metrics',
        title: metric.label ?? metric.name,
        subtitle: metric.description ?? `${metric.aggregation}(${metric.expression})`,
        modelName,
        synonyms: metric.synonyms ?? [],
        searchText: searchable([
          metric.name,
          metric.label,
          metric.description,
          metric.expression,
          metric.aggregation,
          modelName,
          ...(metric.synonyms ?? []),
        ]),
        metric,
      })
    }
    for (const dimension of model.dimensions ?? []) {
      if (dimension.is_active === false) {
        continue
      }
      items.push({
        id: `dimension-${model.id}-${dimension.id}`,
        category: 'dimensions',
        title: dimension.label ?? dimension.name,
        subtitle: dimension.description ?? dimension.column_ref,
        modelName,
        synonyms: dimension.synonyms ?? [],
        searchText: searchable([
          dimension.name,
          dimension.label,
          dimension.description,
          dimension.column_ref,
          dimension.type,
          modelName,
          ...(dimension.synonyms ?? []),
        ]),
        dimension,
      })
    }
  }
  return items
}

export function buildAnswerItems(answers: VerifiedAnswer[]): KnowledgeItem[] {
  return answers.map((answer) => ({
    id: `answer-${answer.id}`,
    category: 'answers' as const,
    title: answer.question,
    subtitle: answer.description ?? answer.name ?? '',
    modelName: '',
    synonyms: answer.tags,
    searchText: searchable([answer.question, answer.name, answer.description, ...answer.tags]),
    answer,
  }))
}

export function filterKnowledgeItems(
  items: KnowledgeItem[],
  category: KnowledgeCategory,
  query: string,
): KnowledgeItem[] {
  const q = query.trim().toLocaleLowerCase()
  return items.filter(
    (item) => item.category === category && (q === '' || item.searchText.includes(q)),
  )
}
