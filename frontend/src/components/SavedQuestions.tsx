import { useMemo, useState } from 'react'
import type { LogicalQuery } from '../types/ai'
import { useT, type TranslationKey } from '../i18n'
import { EmptyState } from './ui/EmptyState'

interface SavedQuestion {
  id: string
  name: string
  description: string
  datasource_id: string
  model_id: string
  logical_query: Partial<LogicalQuery>
  created_at: string
  tags: string[]
}

interface FewShotEntry {
  questionId: string
  savedAt: string
}

const FEWSHOT_STORAGE_KEY = 'biqly_saved_fewshot'

function loadFewShotEntries(): FewShotEntry[] {
  try {
    return JSON.parse(localStorage.getItem(FEWSHOT_STORAGE_KEY) || '[]')
  } catch { return [] }
}

function saveFewShotEntries(entries: FewShotEntry[]) {
  localStorage.setItem(FEWSHOT_STORAGE_KEY, JSON.stringify(entries))
}

function buildDemoQuestions(t: (k: TranslationKey, p?: Record<string, string | number>) => string): SavedQuestion[] {
  return [
    {
      id: '1',
      name: t('saved_questions.demo_q1_name'),
      description: t('saved_questions.demo_q1_desc'),
      datasource_id: 'ds_1',
      model_id: 'orders',
      logical_query: {
        select: [{ type: 'dimension', name: 'country' }, { type: 'metric', name: 'revenue' }],
        group_by: [{ field: 'country' }],
        order_by: [{ field: 'revenue', direction: 'desc' }],
        limit: 100,
      },
      created_at: '2026-05-01',
      tags: ['revenue', 'country'],
    },
    {
      id: '2',
      name: t('saved_questions.demo_q2_name'),
      description: t('saved_questions.demo_q2_desc'),
      datasource_id: 'ds_1',
      model_id: 'users',
      logical_query: {
        select: [{ type: 'dimension', name: 'date' }, { type: 'metric', name: 'user_count' }],
        group_by: [{ field: 'date' }],
        limit: 7,
      },
      created_at: '2026-05-05',
      tags: ['users', 'weekly'],
    },
    {
      id: '3',
      name: t('saved_questions.demo_q3_name'),
      description: t('saved_questions.demo_q3_desc'),
      datasource_id: 'ds_1',
      model_id: 'products',
      logical_query: {
        select: [{ type: 'dimension', name: 'name' }, { type: 'metric', name: 'sales_count' }],
        group_by: [{ field: 'name' }],
        order_by: [{ field: 'sales_count', direction: 'desc' }],
        limit: 10,
      },
      created_at: '2026-05-08',
      tags: ['products', 'sales'],
    },
  ]
}

export default function SavedQuestions() {
  const t = useT()
  const questions = useMemo(() => buildDemoQuestions(t), [t])
  const [search, setSearch] = useState('')
  const [selectedQuestion, setSelectedQuestion] = useState<SavedQuestion | null>(null)
  const [fewShotEntries, setFewShotEntries] = useState<FewShotEntry[]>(loadFewShotEntries)

  const filtered = questions.filter(
    (q) =>
      q.name.toLowerCase().includes(search.toLowerCase()) ||
      q.tags.some((t) => t.toLowerCase().includes(search.toLowerCase()))
  )

  const isFewShot = (id: string) => fewShotEntries.some((e) => e.questionId === id)

  const toggleFewShot = (id: string) => {
    const exists = isFewShot(id)
    const updated = exists
      ? fewShotEntries.filter((e) => e.questionId !== id)
      : [...fewShotEntries, { questionId: id, savedAt: new Date().toISOString() }]
    setFewShotEntries(updated)
    saveFewShotEntries(updated)
  }

  const fewShotCount = fewShotEntries.length

  return (
    <div className="page-stack">
      <div className="card">
        <h2>{t('saved_questions.title')}</h2>
        <p className="saved-question-intro">
          {fewShotCount > 0
            ? t('saved_questions.intro_fewshot_active', { count: fewShotCount })
            : t('saved_questions.intro_fewshot_none')}
        </p>
        <div className="form-group">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('saved_questions.search_placeholder')}
            aria-label={t('saved_questions.search_placeholder')}
            name="saved_question_search"
            autoComplete="off"
          />
        </div>
      </div>

      <div className="saved-question-list">
        <div className="card">
          {filtered.length === 0 ? (
            <EmptyState
              description={search.trim() ? t('saved_questions.no_matches') : t('saved_questions.empty')}
            />
          ) : (
            filtered.map((q) => {
              const checked = isFewShot(q.id)
              return (
              <div key={q.id} className="saved-question-row">
                <button
                  type="button"
                  className="saved-question-item"
                  onClick={() => setSelectedQuestion(q)}
                >
                  <h3>{q.name}</h3>
                  <p>{q.description}</p>
                  <div className="saved-question-tags">
                    {q.tags.map((tag) => (
                      <span key={tag} className="tag-pill">{tag}</span>
                    ))}
                  </div>
                </button>
                <label
                  className={`fewshot-checkbox${checked ? ' is-active' : ''}`}
                  title={t('saved_questions.fewshot_use_title')}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleFewShot(q.id)}
                    onClick={(e) => e.stopPropagation()}
                    aria-label={t('saved_questions.fewshot_aria', { name: q.name })}
                  />
                  <span>{t('saved_questions.fewshot_badge')}</span>
                </label>
              </div>
              )
            })
          )}
        </div>

        {selectedQuestion && (
          <div className="card">
            <h2>{selectedQuestion.name}</h2>
            <p className="saved-question-description">
              {selectedQuestion.description}
            </p>
            <h3>{t('saved_questions.logical_query_heading')}</h3>
            <div className="sql-preview">
              {JSON.stringify(selectedQuestion.logical_query, null, 2)}
            </div>
            <div className="saved-question-actions">
              <button type="button" className="btn" aria-label={t('saved_questions.aria_run_query')}>
                {t('saved_questions.run_query')}
              </button>
              <button type="button" className="btn btn--neutral" aria-label={t('saved_questions.aria_edit_query')}>
                {t('saved_questions.edit_query')}
              </button>
              <button type="button" className="btn btn--destructive" aria-label={t('saved_questions.aria_delete_query')}>
                {t('saved_questions.delete_query')}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
