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
  className: 'w-[1.3rem] h-[1.3rem]',
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
    <div className="flex flex-col gap-7">
      <section>
        <h2 className="m-0 mb-[0.85rem] text-[1rem] font-semibold tracking-[-0.01em]">
          {t('home.quick_actions')}
        </h2>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-[0.85rem]">
          {QUICK_ACTIONS.map((action) => (
            <button
              key={action.path}
              type="button"
              className={`flex flex-col items-start gap-[0.65rem] border border-border rounded-[0.7rem] bg-card p-4 cursor-pointer text-foreground text-left transition-all duration-140 ease-out hover:border-accent hover:bg-[var(--accent-glow)] hover:-translate-y-0.5`}
              onClick={() => {
                void navigate(action.path)
              }}
            >
              <span
                className="inline-grid place-items-center w-[2.2rem] h-[2.2rem] rounded-[0.55rem] bg-[var(--accent-glow)] text-accent"
                aria-hidden="true"
              >
                {action.icon}
              </span>
              <span className="text-[0.9rem] font-semibold">{t(action.labelKey)}</span>
            </button>
          ))}
        </div>
      </section>

      <div className="grid grid-cols-[repeat(auto-fit,minmax(280px,1fr))] gap-5 items-start">
        <RecentQueries />
        <Favorites />
      </div>
    </div>
  )
}

function ListSkeleton() {
  return (
    <div className="flex flex-col gap-[0.3rem] m-0 p-0 list-none">
      {Array.from({ length: 4 }, (_, i) => (
        <div
          key={i}
          className="flex flex-col gap-[0.4rem] w-full rounded-[0.5rem] py-[0.6rem] px-[0.65rem]"
        >
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
    void get<{ entries: RecentQuery[] }>('/api/ai/query/history?limit=8').then((data) => {
      setItems(data?.entries ?? [])
    })
  }, [get])

  return (
    <section className="card min-w-0">
      <h2 className="m-0 mb-[0.85rem] text-[1rem] font-semibold tracking-[-0.01em]">
        {t('home.recent_queries')}
      </h2>
      {items === null ? (
        <ListSkeleton />
      ) : items.length === 0 ? (
        <EmptyState
          title={t('home.recent_empty_title')}
          description={t('home.recent_empty_desc')}
          action={{
            label: t('home.recent_empty_cta'),
            onClick: () => {
              void navigate('/ai-query')
            },
          }}
        />
      ) : (
        <ul className="flex flex-col gap-[0.3rem] m-0 p-0 list-none">
          {items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className="flex flex-col gap-[0.2rem] w-full rounded-[0.5rem] py-[0.6rem] px-[0.65rem] border-0 bg-transparent cursor-pointer text-left text-foreground font-inherit hover:bg-card-raised"
                onClick={() => {
                  void navigate('/ai-query', { state: { question: item.question } })
                }}
                aria-label={`${t('home.open_aria')}: ${item.question}`}
              >
                <span className="text-[0.88rem] font-medium truncate">{item.question}</span>
                <span className="text-[0.76rem] text-foreground-muted truncate">
                  {formatDate(item.created_at)}
                </span>
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
    void get<SavedQuestion[]>('/api/ai/examples/favorites?limit=8').then((data) => {
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
    <section className="card min-w-0">
      <h2 className="m-0 mb-[0.85rem] text-[1rem] font-semibold tracking-[-0.01em]">
        {t('home.favorites')}
      </h2>
      {items === null ? (
        <ListSkeleton />
      ) : items.length === 0 ? (
        <EmptyState
          title={t('home.favorites_empty_title')}
          description={t('home.favorites_empty_desc')}
          action={{
            label: t('home.favorites_empty_cta'),
            onClick: () => {
              void navigate('/saved')
            },
          }}
        />
      ) : (
        <ul className="flex flex-col gap-[0.3rem] m-0 p-0 list-none">
          {items.map((item) => (
            <li key={item.id} className="flex items-center gap-[0.35rem]">
              <button
                type="button"
                className="flex flex-col gap-[0.2rem] w-full rounded-[0.5rem] py-[0.6rem] px-[0.65rem] border-0 bg-transparent cursor-pointer text-left text-foreground font-inherit hover:bg-card-raised min-w-0 flex-1"
                onClick={() => {
                  void navigate('/saved')
                }}
                aria-label={`${t('home.open_aria')}: ${item.name}`}
              >
                <span className="text-[0.88rem] font-medium truncate">{item.name}</span>
                {item.description && (
                  <span className="text-[0.76rem] text-foreground-muted truncate">
                    {item.description}
                  </span>
                )}
              </button>
              <button
                type="button"
                className="shrink-0 border-0 bg-transparent text-warning cursor-pointer text-[1.1rem] leading-none p-[0.3rem] rounded-[0.4rem] hover:bg-card-raised"
                onClick={() => {
                  void unfavorite(item)
                }}
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
