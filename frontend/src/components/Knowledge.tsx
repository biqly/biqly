import { useEffect, useMemo, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useDatasources } from '../hooks/useDatasources'
import { useT } from '../i18n'
import { cardClass, cardIntroClass, cardLeadClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import {
  savedQuestionItemMetaPClass,
  savedQuestionItemTitleClass,
  savedQuestionListClass,
} from '../lib/savedQuestionClasses'
import { toggleGroupClass } from '../lib/toggleClasses'
import type { BusinessGlossaryTerm } from '../types/glossary'
import type { SemanticModelDetail, SemanticModelSummary } from '../types/semantic'
import { pickValidIdOrFirst } from '../utils/effectiveSelection'
import { KnowledgeDetail } from './knowledge/KnowledgeDetail'
import type { KnowledgeCategory, KnowledgeItem, VerifiedAnswer } from './knowledge/types'
import {
  buildAnswerItems,
  buildModelItems,
  buildTermItems,
  filterKnowledgeItems,
  KNOWLEDGE_CATEGORIES,
} from './knowledge/types'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'
import { ToggleButtonGroup } from './ui/ToggleButtonGroup'

export default function Knowledge() {
  const t = useT()
  const { get, loading, error } = useApi()
  const { datasources, loading: dsLoading } = useDatasources()
  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const [category, setCategory] = useState<KnowledgeCategory>('terms')
  const [search, setSearch] = useState('')
  const [items, setItems] = useState<KnowledgeItem[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      if (!datasourceId) {
        setItems([])
        return
      }
      const ds = encodeURIComponent(datasourceId)
      const [terms, models, answers] = await Promise.all([
        get<BusinessGlossaryTerm[]>(`/api/ai/glossary?datasource_id=${ds}`),
        get<SemanticModelSummary[]>(`/api/semantic/models?datasource_id=${ds}`),
        get<VerifiedAnswer[]>(`/api/ai/examples?datasource_id=${ds}`),
      ])
      const details = await Promise.all(
        (models ?? []).map((m) =>
          get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(m.id)}`),
        ),
      )
      if (cancelled) {
        return
      }
      const modelDetails = details.filter((d): d is SemanticModelDetail => d != null)
      setItems([
        ...buildTermItems(terms ?? []),
        ...buildModelItems(modelDetails),
        ...buildAnswerItems(answers ?? []),
      ])
      setSelectedId(null)
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [datasourceId, get])

  const filtered = useMemo(
    () => filterKnowledgeItems(items, category, search),
    [items, category, search],
  )
  const selected = filtered.find((item) => item.id === selectedId) ?? null

  const counts = useMemo(() => {
    const map = new Map<KnowledgeCategory, number>()
    for (const item of items) {
      map.set(item.category, (map.get(item.category) ?? 0) + 1)
    }
    return map
  }, [items])

  if (dsLoading && items.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={cardClass()}>
        <div className={cardIntroClass}>
          <h2>{t('knowledge.title')}</h2>
          <p className={cardLeadClass}>{t('knowledge.intro')}</p>
        </div>

        <div className={cn(formRowClass, 'mt-5')}>
          <div className={legacyFormClass('form-field')} style={{ minWidth: '14rem' }}>
            <label htmlFor="knowledge-datasource" className={legacyFormClass('form-label')}>
              {t('knowledge.label_datasource')}
            </label>
            <Select
              id="knowledge-datasource"
              value={datasourceId}
              onChange={setSelectedDatasourceId}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className={legacyFormClass('form-field')} style={{ flexGrow: 1, minWidth: '16rem' }}>
            <label htmlFor="knowledge-search" className={legacyFormClass('form-label')}>
              {t('common.search')}
            </label>
            <input
              id="knowledge-search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('knowledge.search_placeholder')}
              autoComplete="off"
            />
          </div>
        </div>

        <div className="mt-4">
          <ToggleButtonGroup
            ariaLabel={t('knowledge.category_label')}
            className={toggleGroupClass('flex-wrap')}
            toggleButtons
            value={category}
            onChange={(next) => {
              setCategory(next)
              setSelectedId(null)
            }}
            options={KNOWLEDGE_CATEGORIES.map((c) => ({
              value: c,
              label: `${t(`knowledge.category_${c}`)} (${counts.get(c) ?? 0})`,
            }))}
          />
        </div>
      </div>

      {error && <ErrorAlert error={error} />}

      {filtered.length === 0 ? (
        <div className={cardClass()} style={{ position: 'relative', minHeight: '200px' }}>
          <LoadingOverlay loading={loading} />
          <EmptyState
            description={search.trim() ? t('knowledge.no_matches') : t('knowledge.empty')}
          />
        </div>
      ) : (
        <div className={cn(savedQuestionListClass(), 'items-start')}>
          <div
            className={cn(cardClass(), 'max-h-[70vh] overflow-y-auto')}
            style={{ position: 'relative' }}
          >
            <LoadingOverlay loading={loading} />
            <div className="flex flex-col gap-3">
              {filtered.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={cn(
                    'border-border hover:bg-card w-full rounded-lg border p-3 text-left transition-colors',
                    selected?.id === item.id && 'border-primary bg-card',
                  )}
                  onClick={() => setSelectedId(item.id)}
                >
                  <span className={savedQuestionItemTitleClass()}>{item.title}</span>
                  {item.subtitle && (
                    <p className={savedQuestionItemMetaPClass()}>{item.subtitle}</p>
                  )}
                </button>
              ))}
            </div>
          </div>

          <div className={cn(cardClass(), 'sticky top-6')}>
            <KnowledgeDetail item={selected} t={t} />
          </div>
        </div>
      )}
    </div>
  )
}
