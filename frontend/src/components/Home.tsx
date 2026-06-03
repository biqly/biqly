import '../styles/home.css'

import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useApi } from '../hooks/useApi'
import { useToast } from '../hooks/useToast'
import { type TranslationKey, useT } from '../i18n'
import type { SavedQuestion } from './savedQuestions/types'
import { EmptyState } from './ui/EmptyState'
import { Skeleton } from './ui/Skeleton'

interface RecentQuery {
  id: string
  question: string
  datasource_id: string
  outcome_status: string
  confidence_score?: number
  created_at: string
}

const iconProps = {
  viewBox: '0 0 24 24',
  fill: 'none' as const,
  stroke: 'currentColor',
  strokeWidth: 1.6,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  'aria-hidden': true,
}

interface QuickAction {
  path: string
  labelKey: TranslationKey
  icon: React.ReactNode
}

const QUICK_ACTIONS: QuickAction[] = [
  {
    path: '/ai-query',
    labelKey: 'app.nav.ai_query',
    icon: (
      <svg {...iconProps}>
        <path d="M12 3.5l1.6 4.4 4.4 1.6-4.4 1.6L12 15.5l-1.6-4.4L6 9.5l4.4-1.6z" />
        <path d="M18.5 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z" />
      </svg>
    ),
  },
  {
    path: '/query-builder',
    labelKey: 'app.nav.query_builder',
    icon: (
      <svg {...iconProps}>
        <rect x="3.5" y="3.5" width="7" height="7" rx="1.4" />
        <rect x="13.5" y="3.5" width="7" height="7" rx="1.4" />
        <rect x="3.5" y="13.5" width="7" height="7" rx="1.4" />
        <rect x="13.5" y="13.5" width="7" height="7" rx="1.4" />
      </svg>
    ),
  },
  {
    path: '/datasources',
    labelKey: 'app.nav.datasources',
    icon: (
      <svg {...iconProps}>
        <ellipse cx="12" cy="5.5" rx="7" ry="2.5" />
        <path d="M5 5.5v6c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5v-6" />
        <path d="M5 11.5v6c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5v-6" />
      </svg>
    ),
  },
  {
    path: '/saved',
    labelKey: 'app.nav.saved_questions',
    icon: (
      <svg {...iconProps}>
        <path d="M6 4.5h12a1 1 0 0 1 1 1v15l-7-4-7 4v-15a1 1 0 0 1 1-1z" />
      </svg>
    ),
  },
  {
    path: '/table-browser',
    labelKey: 'app.nav.table_browser',
    icon: (
      <svg {...iconProps}>
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M3 9h18" />
        <path d="M9 21V9" />
      </svg>
    ),
  },
]

export default function Home() {
  const t = useT()
  const navigate = useNavigate()

  return (
    <div className="home">
      <section className="home-section">
        <h2 className="home-section__title">{t('home.quick_actions')}</h2>
        <div className="home-quick-actions">
          {QUICK_ACTIONS.map((action) => (
            <button
              key={action.path}
              type="button"
              className="home-quick-action"
              onClick={() => navigate(action.path)}
            >
              <span className="home-quick-action__icon" aria-hidden="true">
                {action.icon}
              </span>
              <span className="home-quick-action__label">{t(action.labelKey)}</span>
            </button>
          ))}
        </div>
      </section>

      <div className="home-grid">
        <RecentQueries />
        <Favorites />
      </div>
    </div>
  )
}

function ListSkeleton() {
  return (
    <div className="home-list">
      {Array.from({ length: 4 }, (_, i) => (
        <div key={i} className="home-list-item home-list-item--skeleton">
          <Skeleton height="0.85rem" width={`${70 - i * 8}%`} />
          <Skeleton height="0.7rem" width="40%" />
        </div>
      ))}
    </div>
  )
}

function RecentQueries() {
  const t = useT()
  const navigate = useNavigate()
  const { get } = useApi()
  const [items, setItems] = useState<RecentQuery[] | null>(null)

  useEffect(() => {
    get<{ entries: RecentQuery[] }>('/api/ai/query/history?limit=8').then((data) => {
      setItems(data?.entries ?? [])
    })
  }, [get])

  return (
    <section className="card home-card">
      <h2 className="home-section__title">{t('home.recent_queries')}</h2>
      {items === null ? (
        <ListSkeleton />
      ) : items.length === 0 ? (
        <EmptyState
          title={t('home.recent_empty_title')}
          description={t('home.recent_empty_desc')}
          action={{ label: t('home.recent_empty_cta'), onClick: () => navigate('/ai-query') }}
        />
      ) : (
        <ul className="home-list">
          {items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className="home-list-item home-list-item--button"
                onClick={() => navigate('/ai-query', { state: { question: item.question } })}
                aria-label={`${t('home.open_aria')}: ${item.question}`}
              >
                <span className="home-list-item__title">{item.question}</span>
                <span className="home-list-item__meta">{formatDate(item.created_at)}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function Favorites() {
  const t = useT()
  const navigate = useNavigate()
  const { get, putData } = useApi()
  const toast = useToast()
  const [items, setItems] = useState<SavedQuestion[] | null>(null)

  useEffect(() => {
    get<SavedQuestion[]>('/api/ai/examples/favorites?limit=8').then((data) => {
      setItems(data ?? [])
    })
  }, [get])

  const unfavorite = useCallback(
    async (q: SavedQuestion) => {
      const ok = await putData(`/api/ai/examples/${q.id}`, {
        question: q.question,
        logical_query: q.logical_query,
        tags: q.tags,
        dialect: q.dialect,
        locale: q.locale,
        name: q.name,
        description: q.description,
        is_few_shot: q.is_few_shot,
        is_favorite: false,
      })
      if (ok !== null) {
        setItems((prev) => (prev ? prev.filter((x) => x.id !== q.id) : prev))
        toast.success(t('home.unfavorited_toast'))
      }
    },
    [putData, toast, t],
  )

  return (
    <section className="card home-card">
      <h2 className="home-section__title">{t('home.favorites')}</h2>
      {items === null ? (
        <ListSkeleton />
      ) : items.length === 0 ? (
        <EmptyState
          title={t('home.favorites_empty_title')}
          description={t('home.favorites_empty_desc')}
          action={{ label: t('home.favorites_empty_cta'), onClick: () => navigate('/saved') }}
        />
      ) : (
        <ul className="home-list">
          {items.map((item) => (
            <li key={item.id} className="home-fav-row">
              <button
                type="button"
                className="home-list-item home-list-item--button"
                onClick={() => navigate('/saved')}
                aria-label={`${t('home.open_aria')}: ${item.name}`}
              >
                <span className="home-list-item__title">{item.name}</span>
                {item.description && (
                  <span className="home-list-item__meta">{item.description}</span>
                )}
              </button>
              <button
                type="button"
                className="home-fav-star"
                onClick={() => unfavorite(item)}
                aria-label={t('home.unfavorite')}
                title={t('home.unfavorite')}
              >
                ★
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) {
    return ''
  }
  return d.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
