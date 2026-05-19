import { Suspense, lazy, useEffect, useMemo, useState, type ComponentType, type LazyExoticComponent, type MouseEvent, type ReactNode } from 'react'
import { EmptyState } from './components/ui/EmptyState'
import { ErrorBoundary } from './components/ui/ErrorBoundary'
import { LanguageSwitcher } from './components/ui/LanguageSwitcher'
import { ThemeToggle } from './components/ui/ThemeToggle'
import abiLogo from './assets/abi-logo.png'
import { useT, type TranslationKey } from './i18n'

const Datasources = lazy(() => import('./components/Datasources'))
const Metadata = lazy(() => import('./components/Metadata'))
const Modeling = lazy(() => import('./components/Modeling'))
const QueryBuilder = lazy(() => import('./components/QueryBuilder'))
const AIQuery = lazy(() => import('./components/AIQuery'))
const SavedQuestions = lazy(() => import('./components/SavedQuestions'))
const FewShotExamples = lazy(() => import('./components/FewShotExamples'))
const Evaluation = lazy(() => import('./components/Evaluation'))
const Dashboard = lazy(() => import('./components/Dashboard'))
const Settings = lazy(() => import('./components/Settings'))

type RouteSectionKey = 'data' | 'query' | 'ai' | 'analytics' | 'preferences'

const ROUTE_SECTION_ORDER: RouteSectionKey[] = ['data', 'query', 'ai', 'preferences']

const sectionLabelKeys: Record<RouteSectionKey, TranslationKey> = {
  data: 'app.sections.data',
  query: 'app.sections.query',
  ai: 'app.sections.ai',
  analytics: 'app.sections.analytics',
  preferences: 'app.sections.preferences',
}

interface AppRouteDef {
  path: string
  sectionKey: RouteSectionKey
  labelKey: TranslationKey
  eyebrowKey: TranslationKey
  descriptionKey: TranslationKey
  icon: ReactNode
  component: LazyExoticComponent<ComponentType>
}

interface AppRoute extends AppRouteDef {
  label: string
  eyebrow: string
  description: string
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

const IconDatasources = (
  <svg {...iconProps}>
    <ellipse cx="12" cy="5.5" rx="7" ry="2.5" />
    <path d="M5 5.5v6c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5v-6" />
    <path d="M5 11.5v6c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5v-6" />
  </svg>
)

const IconMetadata = (
  <svg {...iconProps}>
    <path d="M4 4.5h6.5a2 2 0 0 1 2 2V20" />
    <path d="M20 4.5h-6.5a2 2 0 0 0-2 2V20" />
    <path d="M4 4.5V18a1.5 1.5 0 0 0 1.5 1.5h5" />
    <path d="M20 4.5V18a1.5 1.5 0 0 1-1.5 1.5h-5" />
  </svg>
)

const IconModeling = (
  <svg {...iconProps}>
    <rect x="3.5" y="4" width="6.5" height="5.5" rx="1.3" />
    <rect x="14" y="4" width="6.5" height="5.5" rx="1.3" />
    <rect x="8.75" y="15" width="6.5" height="5.5" rx="1.3" />
    <path d="M10 6.8h4" />
    <path d="M16.2 9.5l-2.5 5.5" />
    <path d="M7.8 9.5l2.5 5.5" />
  </svg>
)

const IconQueryBuilder = (
  <svg {...iconProps}>
    <rect x="3.5" y="3.5" width="7" height="7" rx="1.4" />
    <rect x="13.5" y="3.5" width="7" height="7" rx="1.4" />
    <rect x="3.5" y="13.5" width="7" height="7" rx="1.4" />
    <rect x="13.5" y="13.5" width="7" height="7" rx="1.4" />
  </svg>
)

const IconAIQuery = (
  <svg {...iconProps}>
    <path d="M12 3.5l1.6 4.4 4.4 1.6-4.4 1.6L12 15.5l-1.6-4.4L6 9.5l4.4-1.6z" />
    <path d="M18.5 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z" />
  </svg>
)

const IconSaved = (
  <svg {...iconProps}>
    <path d="M6 4.5h12a1 1 0 0 1 1 1v15l-7-4-7 4v-15a1 1 0 0 1 1-1z" />
  </svg>
)

const IconFewShot = (
  <svg {...iconProps}>
    <path d="M9 18h6" />
    <path d="M10 21h4" />
    <path d="M12 3a6 6 0 0 0-3.5 10.9c.5.4.9 1 1 1.6l.2 1h4.6l.2-1c.1-.6.5-1.2 1-1.6A6 6 0 0 0 12 3z" />
  </svg>
)

const IconEvaluation = (
  <svg {...iconProps}>
    <path d="M4 20V10" />
    <path d="M10 20V4" />
    <path d="M16 20v-7" />
    <path d="M22 20H2" />
  </svg>
)

const IconDashboard = (
  <svg {...iconProps}>
    <path d="M4 21V14" />
    <path d="M9.5 21v-8" />
    <path d="M15 21v-5.5" />
    <path d="M20.5 21V8" />
    <path d="M3 21h18" strokeWidth="1.65" />
  </svg>
)

const IconSettings = (
  <svg {...iconProps}>
    <circle cx="7.5" cy="8.5" r="1.5" />
    <path d="M11 8.5h9.5" />
    <circle cx="17" cy="15.5" r="1.5" />
    <path d="M3.5 15.5h11" />
  </svg>
)

const routeDefs: AppRouteDef[] = [
  {
    path: '/datasources',
    sectionKey: 'data',
    labelKey: 'app.nav.datasources',
    eyebrowKey: 'app.nav.datasources_eyebrow',
    descriptionKey: 'app.nav.datasources_desc',
    icon: IconDatasources,
    component: Datasources,
  },
  {
    path: '/metadata',
    sectionKey: 'data',
    labelKey: 'app.nav.metadata',
    eyebrowKey: 'app.nav.metadata_eyebrow',
    descriptionKey: 'app.nav.metadata_desc',
    icon: IconMetadata,
    component: Metadata,
  },
  {
    path: '/modeling',
    sectionKey: 'data',
    labelKey: 'app.nav.modeling',
    eyebrowKey: 'app.nav.modeling_eyebrow',
    descriptionKey: 'app.nav.modeling_desc',
    icon: IconModeling,
    component: Modeling,
  },
  {
    path: '/query-builder',
    sectionKey: 'query',
    labelKey: 'app.nav.query_builder',
    eyebrowKey: 'app.nav.query_builder_eyebrow',
    descriptionKey: 'app.nav.query_builder_desc',
    icon: IconQueryBuilder,
    component: QueryBuilder,
  },
  {
    path: '/ai-query',
    sectionKey: 'query',
    labelKey: 'app.nav.ai_query',
    eyebrowKey: 'app.nav.ai_query_eyebrow',
    descriptionKey: 'app.nav.ai_query_desc',
    icon: IconAIQuery,
    component: AIQuery,
  },
  {
    path: '/saved',
    sectionKey: 'query',
    labelKey: 'app.nav.saved_questions',
    eyebrowKey: 'app.nav.saved_questions_eyebrow',
    descriptionKey: 'app.nav.saved_questions_desc',
    icon: IconSaved,
    component: SavedQuestions,
  },
  {
    path: '/few-shot-examples',
    sectionKey: 'ai',
    labelKey: 'app.nav.few_shot',
    eyebrowKey: 'app.nav.few_shot_eyebrow',
    descriptionKey: 'app.nav.few_shot_desc',
    icon: IconFewShot,
    component: FewShotExamples,
  },
  {
    path: '/evaluation',
    sectionKey: 'ai',
    labelKey: 'app.nav.evaluation',
    eyebrowKey: 'app.nav.evaluation_eyebrow',
    descriptionKey: 'app.nav.evaluation_desc',
    icon: IconEvaluation,
    component: Evaluation,
  },
  {
    path: '/dashboard',
    sectionKey: 'ai',
    labelKey: 'app.nav.dashboard',
    eyebrowKey: 'app.nav.dashboard_eyebrow',
    descriptionKey: 'app.nav.dashboard_desc',
    icon: IconDashboard,
    component: Dashboard,
  },
  {
    path: '/settings',
    sectionKey: 'preferences',
    labelKey: 'app.nav.settings',
    eyebrowKey: 'app.nav.settings_eyebrow',
    descriptionKey: 'app.nav.settings_desc',
    icon: IconSettings,
    component: Settings,
  },
]

const DEFAULT_PATH = routeDefs[0]!.path

const initialPath = () => {
  const { pathname } = window.location
  if (pathname === '/') return DEFAULT_PATH
  const def = routeDefs.find((r) => r.path === pathname)
  return def?.path ?? pathname
}

function App() {
  const t = useT()
  const routes: AppRoute[] = useMemo(
    () =>
      routeDefs.map((def) => ({
        ...def,
        label: t(def.labelKey),
        eyebrow: t(def.eyebrowKey),
        description: t(def.descriptionKey),
      })),
    [t],
  )
  const sidebarSections = useMemo(() => {
    const buckets = new Map<RouteSectionKey, AppRoute[]>()
    for (const route of routes) {
      const prev = buckets.get(route.sectionKey) ?? []
      prev.push(route)
      buckets.set(route.sectionKey, prev)
    }
    const out: { sectionKey: RouteSectionKey; heading: string; routes: AppRoute[] }[] = []
    for (const key of ROUTE_SECTION_ORDER) {
      const sr = buckets.get(key)
      if (sr?.length) {
        out.push({ sectionKey: key, heading: t(sectionLabelKeys[key]), routes: sr })
      }
    }
    return out
  }, [routes, t])
  const findRoute = (pathname: string) => routes.find((route) => route.path === pathname)

  const [activePath, setActivePath] = useState(initialPath)
  const activeRoute = findRoute(activePath)
  const ActiveComponent = activeRoute?.component

  useEffect(() => {
    if (window.location.pathname === '/') {
      window.history.replaceState(null, '', DEFAULT_PATH)
    }
  }, [])

  useEffect(() => {
    const handlePopState = () => setActivePath(initialPath())
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    document.title = activeRoute ? `${activeRoute.label} · ABI` : `${t('common.page_not_found')} · ABI`
  }, [activeRoute, t])

  const navigate = (path: string) => {
    if (path === window.location.pathname) return
    window.history.pushState(null, '', path)
    setActivePath(path)
    window.scrollTo({ top: 0, behavior: 'auto' })
  }

  const handleNavClick = (event: MouseEvent<HTMLAnchorElement>, path: string) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      event.currentTarget.target
    ) {
      return
    }

    const targetUrl = new URL(event.currentTarget.href)
    if (targetUrl.origin !== window.location.origin) return

    event.preventDefault()
    navigate(path)
  }

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        {t('common.skip_to_content')}
      </a>

      <aside className="sidebar" aria-label={t('common.primary_nav')}>
        <a className="brand" href={DEFAULT_PATH} onClick={(event) => handleNavClick(event, DEFAULT_PATH)}>
          <span className="brand-mark" aria-hidden="true">
            <img src={abiLogo} alt="" width={34} height={34} />
          </span>
          <span className="brand-text">
            <strong>ABI</strong>
            <small>{t('common.brand_subtitle')}</small>
          </span>
        </a>

        <div className="sidebar-nav-scroll" role="presentation">
          {sidebarSections.map((section) => (
            <section key={section.sectionKey} className="nav-section" aria-labelledby={`nav-heading-${section.sectionKey}`}>
              <div className="nav-section-label" id={`nav-heading-${section.sectionKey}`}>
                {section.heading}
              </div>
              <div className="nav-section-links">
                {section.routes.map((route) => (
                  <a
                    key={route.path}
                    className="nav-link"
                    href={route.path}
                    aria-current={activeRoute?.path === route.path ? 'page' : undefined}
                    onClick={(event) => handleNavClick(event, route.path)}
                  >
                    <span className="nav-icon" aria-hidden="true">{route.icon}</span>
                    <span className="nav-label">{route.label}</span>
                  </a>
                ))}
              </div>
            </section>
          ))}
        </div>

        <div className="sidebar-footer">
          <div className="header-controls">
            <LanguageSwitcher />
            <ThemeToggle />
          </div>
          <div className="sidebar-footer__api">
            <span className="status-dot" aria-hidden="true" />
            <span>{t('common.local_api')}</span>
          </div>
        </div>
      </aside>

      <main id="main-content" className="main" tabIndex={-1}>
        <header className="page-header">
          <p>{activeRoute?.eyebrow ?? t('common.not_found_eyebrow')}</p>
          <div>
            <h1>{activeRoute?.label ?? t('common.page_not_found')}</h1>
            <span>{activeRoute?.description ?? t('common.not_found_desc')}</span>
          </div>
        </header>

        {ActiveComponent ? (
          <ErrorBoundary key={activeRoute.path}>
            <Suspense
              fallback={
                <section className="card card--elevated">
                  <EmptyState description={t('common.module_loading')} />
                </section>
              }
            >
              <ActiveComponent />
            </Suspense>
          </ErrorBoundary>
        ) : (
          <section className="card card--elevated">
            <EmptyState title={t('common.module_not_found')} description={t('common.module_not_found_desc')}>
              <a className="btn" href={DEFAULT_PATH} onClick={(event) => handleNavClick(event, DEFAULT_PATH)}>
                {t('common.go_to_datasources')}
              </a>
            </EmptyState>
          </section>
        )}
      </main>
    </div>
  )
}

export default App
