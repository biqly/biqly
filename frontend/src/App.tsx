import { Suspense, lazy, useEffect, useMemo, useState, startTransition, type ComponentType, type LazyExoticComponent, type MouseEvent, type ReactNode } from 'react'
import { Routes, Route, Outlet, Link, useLocation, useNavigate } from 'react-router-dom'
import { Breadcrumbs, type Crumb } from './components/ui/Breadcrumbs'
import { CommandPalette, type CommandItem } from './components/ui/CommandPalette'
import { EmptyState } from './components/ui/EmptyState'
import { ErrorBoundary } from './components/ui/ErrorBoundary'
import { LanguageSwitcher } from './components/ui/LanguageSwitcher'
import { ThemeToggle } from './components/ui/ThemeToggle'
import abiLogo from './assets/abi-logo.png'
import { useT, LocaleSection, useLocaleSection, type TranslationKey } from './i18n'
import { LoadingScreen } from './components/ui/LoadingScreen'

const lazyWithPreload = <T extends ComponentType<any>>(
  factory: () => Promise<{ default: T }>
) => {
  const Component = lazy(factory) as any
  Component.preload = factory
  return Component as LazyExoticComponent<T> & { preload: () => Promise<{ default: T }> }
}

const Home = lazyWithPreload(() => import('./components/Home'))
const Datasources = lazyWithPreload(() => import('./components/Datasources'))
const Metadata = lazyWithPreload(() => import('./components/Metadata'))
const Modeling = lazyWithPreload(() => import('./components/Modeling'))
const QueryBuilder = lazyWithPreload(() => import('./components/QueryBuilder'))
const AIQuery = lazyWithPreload(() => import('./components/AIQuery'))
const SavedQuestions = lazyWithPreload(() => import('./components/SavedQuestions'))
const TableBrowser = lazyWithPreload(() => import('./components/TableBrowser'))
const QueryHistory = lazyWithPreload(() => import('./components/QueryHistory'))
const FewShotExamples = lazyWithPreload(() => import('./components/FewShotExamples'))
const PromptTemplates = lazyWithPreload(() => import('./components/PromptTemplates'))
const Glossary = lazyWithPreload(() => import('./components/Glossary'))
const Evaluation = lazyWithPreload(() => import('./components/Evaluation'))
const Dashboard = lazyWithPreload(() => import('./components/Dashboard'))
const Settings = lazyWithPreload(() => import('./components/Settings'))
const TimeGrains = lazyWithPreload(() => import('./components/TimeGrains'))
const Admin = lazyWithPreload(() => import('./components/admin/Admin'))
const SignInPage = lazyWithPreload(() => import('./components/auth/SignInPage'))
const SignUpPage = lazyWithPreload(() => import('./components/auth/SignUpPage'))
const ForgotPasswordPage = lazyWithPreload(() => import('./components/auth/ForgotPasswordPage'))
const ResetPasswordPage = lazyWithPreload(() => import('./components/auth/ResetPasswordPage'))
const VerifyEmailPage = lazyWithPreload(() => import('./components/auth/VerifyEmailPage'))
const OAuthCallback = lazyWithPreload(() => import('./components/auth/OAuthCallback'))
const ClaimInvitePage = lazyWithPreload(() => import('./components/auth/ClaimInvitePage'))

import { AuthGuard, GuestGuard } from './components/auth/AuthGuard'
import { useAuth } from './components/auth/AuthProvider'
import { WorkspaceSelector } from './components/workspaces/WorkspaceSelector'
import { useUrlSearch } from './hooks/useQueryParam'
import { appendAdminBreadcrumbs } from './lib/adminBreadcrumbs'
import './styles/sidebar.css'


type RouteSectionKey = 'data' | 'query' | 'ai' | 'analytics' | 'preferences'
type NavigateFn = (path: string) => void
type RouteComponentProps = {
  navigate?: NavigateFn
}

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
  component: LazyExoticComponent<ComponentType<RouteComponentProps>>
  hidden?: boolean
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

const IconHome = (
  <svg {...iconProps}>
    <path d="M4 11.5 12 4l8 7.5" />
    <path d="M6 10v9a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-9" />
    <path d="M10 20v-5h4v5" />
  </svg>
)

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

const IconTableBrowser = (
  <svg {...iconProps}>
    <rect x="3" y="3" width="18" height="18" rx="2" />
    <path d="M3 9h18" />
    <path d="M9 21V9" />
  </svg>
)

const IconFewShot = (
  <svg {...iconProps}>
    <path d="M9 18h6" />
    <path d="M10 21h4" />
    <path d="M12 3a6 6 0 0 0-3.5 10.9c.5.4.9 1 1 1.6l.2 1h4.6l.2-1c.1-.6.5-1.2 1-1.6A6 6 0 0 0 12 3z" />
  </svg>
)

const IconPromptTemplates = (
  <svg {...iconProps}>
    <path d="M7 4.5h10a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-13a1 1 0 0 1 1-1z" />
    <path d="M9 8.5h6" />
    <path d="M9 12h6" />
    <path d="M9 15.5h4" />
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

const IconGlossary = (
  <svg {...iconProps}>
    <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1-2.5-2.5z" />
    <path d="M6 6h10" />
    <path d="M6 10h10" />
    <path d="M6 14h10" />
  </svg>
)

const IconHistory = (
  <svg {...iconProps}>
    <circle cx="12" cy="12" r="10" />
    <path d="M12 6v6l4 2" />
  </svg>
)

const routeDefs: AppRouteDef[] = [
  {
    path: '/',
    sectionKey: 'data',
    labelKey: 'app.nav.home',
    eyebrowKey: 'app.nav.home_eyebrow',
    descriptionKey: 'app.nav.home_desc',
    icon: IconHome,
    component: Home,
    hidden: true,
  },
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
    path: '/table-browser',
    sectionKey: 'query',
    labelKey: 'app.nav.table_browser',
    eyebrowKey: 'app.nav.table_browser_eyebrow',
    descriptionKey: 'app.nav.table_browser_desc',
    icon: IconTableBrowser,
    component: TableBrowser,
  },
  {
    path: '/query-history',
    sectionKey: 'query',
    labelKey: 'app.nav.query_history',
    eyebrowKey: 'app.nav.query_history_eyebrow',
    descriptionKey: 'app.nav.query_history_desc',
    icon: IconHistory,
    component: QueryHistory,
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
    path: '/glossary',
    sectionKey: 'ai',
    labelKey: 'app.nav.glossary',
    eyebrowKey: 'app.nav.glossary_eyebrow',
    descriptionKey: 'app.nav.glossary_desc',
    icon: IconGlossary,
    component: Glossary,
  },
  {
    path: '/prompt-templates',
    sectionKey: 'ai',
    labelKey: 'app.nav.prompt_templates',
    eyebrowKey: 'app.nav.prompt_templates_eyebrow',
    descriptionKey: 'app.nav.prompt_templates_desc',
    icon: IconPromptTemplates,
    component: PromptTemplates,
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
  {
    path: '/time-grains',
    sectionKey: 'preferences',
    labelKey: 'app.nav.time_grains',
    eyebrowKey: 'app.nav.time_grains_eyebrow',
    descriptionKey: 'app.nav.time_grains_desc',
    icon: IconSettings,
    component: TimeGrains,
    hidden: true,
  },
  {
    path: '/admin',
    sectionKey: 'preferences',
    labelKey: 'app.nav.admin',
    eyebrowKey: 'app.nav.admin_eyebrow',
    descriptionKey: 'app.nav.admin_desc',
    icon: IconSettings,
    component: Admin,
    hidden: true,
  },
]

const DEFAULT_PATH = routeDefs[0]!.path

const AuthLoading = () => {
  return (
    <div className="auth-page">
      <div className="auth-card">
        <LoadingScreen minHeight="auto" variant="center" />
      </div>
    </div>
  )
}

function App() {
  const t = useT()
  const authReady = useLocaleSection('auth')
  const adminReady = useLocaleSection('admin')
  const location = useLocation()
  const urlSearch = useUrlSearch()
  const navigate = useNavigate()
  const { user, accessToken, logout, roles } = useAuth()
  const isAdmin = roles.some((role) => role === 'super_admin' || role === 'admin')
  const roleLabel = roles.includes('super_admin')
    ? 'Super Admin'
    : roles.includes('admin')
      ? 'Admin'
      : roles.includes('developer')
        ? 'Developer'
        : roles.includes('analyst')
          ? 'Analyst'
          : 'User'

  const getInitials = (name?: string, email?: string) => {
    if (name) {
      const parts = name.split(' ')
      const first = parts[0]
      const second = parts[1]
      if (first && second && first[0] && second[0]) {
        return (first[0] + second[0]).toUpperCase()
      }
      return name.slice(0, 2).toUpperCase()
    }
    if (email) {
      return email.slice(0, 2).toUpperCase()
    }
    return 'U'
  }

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
      if (route.hidden && !(route.path === '/admin' && isAdmin)) continue
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
  }, [routes, t, isAdmin])

  const homeRoute = useMemo(() => routes.find((r) => r.path === '/'), [routes])

  const commandItems = useMemo<CommandItem[]>(() => {
    const sectionItems = sidebarSections.flatMap((section) =>
      section.routes.map((route) => ({
        id: route.path,
        label: route.label,
        group: section.heading,
        keywords: `${route.eyebrow} ${route.description} ${route.path}`,
        icon: route.icon,
        perform: () => startTransition(() => navigate(route.path)),
      })),
    )
    if (homeRoute) {
      sectionItems.unshift({
        id: homeRoute.path,
        label: homeRoute.label,
        group: homeRoute.eyebrow,
        keywords: `${homeRoute.eyebrow} ${homeRoute.description} ${homeRoute.path}`,
        icon: homeRoute.icon,
        perform: () => startTransition(() => navigate(homeRoute.path)),
      })
    }
    return sectionItems
  }, [sidebarSections, homeRoute, navigate])

  const activePath = location.pathname
  const activeRoute = useMemo(() => {
    return routes.find((route) => {
      if (route.path === activePath) return true
      if (route.path === '/modeling' && (activePath.startsWith('/modeling/') || activePath.startsWith('/model/'))) return true
      return false
    })
  }, [routes, activePath])

  const breadcrumbs = useMemo<Crumb[]>(() => {
    if (!activeRoute) return []
    const section = sidebarSections.find((s) => s.routes.some((r) => r.path === activeRoute.path))
    const crumbs: Crumb[] = []
    if (section) crumbs.push({ label: section.heading })

    const usesUrlSync = activeRoute.path === '/admin' || activeRoute.path === '/evaluation'
    const effectiveSearch = usesUrlSync ? urlSearch : location.search
    const hasAdminOrEvalTab = usesUrlSync && new URLSearchParams(effectiveSearch).has('tab')

    crumbs.push({
      label: activeRoute.label,
      onClick: hasAdminOrEvalTab
        ? () => {
            const next = new URLSearchParams()
            next.set('tab', new URLSearchParams(effectiveSearch).get('tab') || 'users')
            startTransition(() => {
              navigate(`${activeRoute.path}?${next.toString()}`)
            })
          }
        : () => startTransition(() => navigate(activeRoute.path)),
    })

    if (activeRoute.path === '/admin') {
      appendAdminBreadcrumbs(crumbs, effectiveSearch, t, (p) => startTransition(() => navigate(p)))
    } else if (activeRoute.path === '/evaluation') {
      const searchParams = new URLSearchParams(effectiveSearch)
      const tabParam = searchParams.get('tab') || 'run'

      let tabLabel = ''
      if (tabParam === 'run') tabLabel = t('evaluation.tab_run')
      else if (tabParam === 'history') tabLabel = t('evaluation.tab_history')
      else if (tabParam === 'regression') tabLabel = t('evaluation.tab_regression')

      if (tabLabel) {
        crumbs.push({
          label: tabLabel,
          onClick: () => startTransition(() => navigate(`/evaluation?tab=${tabParam}`)),
        })
      }
    }

    return crumbs
  }, [activeRoute, sidebarSections, location.search, urlSearch, navigate, t])

  const [mobileNavOpen, setMobileNavOpen] = useState(false)

  useEffect(() => {
    if (!mobileNavOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMobileNavOpen(false)
    }
    window.addEventListener('keydown', onKey)
    document.body.classList.add('app-shell--nav-open')
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.classList.remove('app-shell--nav-open')
    }
  }, [mobileNavOpen])

  useEffect(() => {
    if (activeRoute) {
      document.title = `${activeRoute.label} · ABI`
    } else if (activePath === '/auth/signin') {
      document.title = t('auth.title_signin')
    } else if (activePath === '/auth/signup') {
      document.title = `${t('auth.title_signup')} · ABI`
    } else if (activePath === '/auth/forgot-password') {
      document.title = `${t('auth.title_forgot')} · ABI`
    } else if (activePath === '/auth/reset-password') {
      document.title = `${t('auth.title_reset')} · ABI`
    } else if (activePath === '/auth/verify-email') {
      document.title = `${t('auth.title_verify')} · ABI`
    } else if (activePath === '/auth/claim-invite') {
      document.title = `${t('auth.title_invite')} · ABI`
    } else if (activePath.startsWith('/auth/')) {
      document.title = 'ABI'
    } else {
      document.title = `${t('common.page_not_found')} · ABI`
    }
  }, [activeRoute, activePath, t])

  if (!authReady || !adminReady) {
    return <LoadingScreen />
  }

  const handleNavHover = (component: any) => {
    if (component && typeof component.preload === 'function') {
      component.preload()
    }
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      routes.forEach((route) => {
        if ((route.component as any).preload) {
          ;(route.component as any).preload()
        }
      })
    }, 1000)
    return () => clearTimeout(timer)
  }, [routes])

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
    startTransition(() => {
      navigate(path)
    })
    setMobileNavOpen(false)
  }

  return (
    <Routes>
      <Route
        element={
          <GuestGuard>
            <LocaleSection name="auth" fallback={<AuthLoading />}>
              <Outlet />
            </LocaleSection>
          </GuestGuard>
        }
      >
        <Route path="/auth/signin" element={<Suspense fallback={<AuthLoading />}><SignInPage /></Suspense>} />
        <Route path="/auth/signup" element={<Suspense fallback={<AuthLoading />}><SignUpPage /></Suspense>} />
        <Route path="/auth/forgot-password" element={<Suspense fallback={<AuthLoading />}><ForgotPasswordPage /></Suspense>} />
        <Route path="/auth/reset-password" element={<Suspense fallback={<AuthLoading />}><ResetPasswordPage /></Suspense>} />
        <Route path="/auth/verify-email" element={<Suspense fallback={<AuthLoading />}><VerifyEmailPage /></Suspense>} />
        <Route path="/auth/claim-invite" element={<Suspense fallback={<AuthLoading />}><ClaimInvitePage /></Suspense>} />
        <Route path="/auth/callback" element={<Suspense fallback={<AuthLoading />}><OAuthCallback /></Suspense>} />
      </Route>

      <Route
        path="*"
        element={
          <AuthGuard>
            <div className={`app-shell${mobileNavOpen ? ' app-shell--nav-open' : ''}`}>
              <CommandPalette items={commandItems} />
              <a className="skip-link" href="#main-content">
                {t('common.skip_to_content')}
              </a>

              <button
                type="button"
                className="mobile-nav-toggle"
                aria-label={mobileNavOpen ? t('common.close_menu') : t('common.open_menu')}
                aria-expanded={mobileNavOpen}
                aria-controls="primary-sidebar"
                onClick={() => setMobileNavOpen((v) => !v)}
              >
                <span aria-hidden="true">{mobileNavOpen ? '✕' : '☰'}</span>
              </button>

              <div
                className="mobile-nav-backdrop"
                hidden={!mobileNavOpen}
                onClick={() => setMobileNavOpen(false)}
                aria-hidden="true"
              />

              <aside id="primary-sidebar" className="sidebar" aria-label={t('common.primary_nav')}>
                <a
                  className="brand"
                  href={DEFAULT_PATH}
                  onClick={(event) => handleNavClick(event, DEFAULT_PATH)}
                  onMouseEnter={() => handleNavHover(Home)}
                  onFocus={() => handleNavHover(Home)}
                >
                  <span className="brand-mark" aria-hidden="true">
                    <img src={abiLogo} alt="" width={34} height={34} />
                  </span>
                  <span className="brand-text">
                    <strong>ABI</strong>
                    <small>{t('common.brand_subtitle')}</small>
                  </span>
                </a>

                <div className="sidebar-nav-scroll" role="presentation">
                  {accessToken && <WorkspaceSelector token={accessToken} />}
                  {homeRoute && (
                    <div className="nav-section-links nav-section-links--home">
                      <a
                        className="nav-link"
                        href={homeRoute.path}
                        aria-current={activeRoute?.path === homeRoute.path ? 'page' : undefined}
                        onClick={(event) => handleNavClick(event, homeRoute.path)}
                        onMouseEnter={() => handleNavHover(homeRoute.component)}
                        onFocus={() => handleNavHover(homeRoute.component)}
                      >
                        <span className="nav-icon" aria-hidden="true">{homeRoute.icon}</span>
                        <span className="nav-label">{homeRoute.label}</span>
                      </a>
                    </div>
                  )}
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
                            onMouseEnter={() => handleNavHover(route.component)}
                            onFocus={() => handleNavHover(route.component)}
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
                  {user && (
                    <>
                      <Link
                        to="/settings"
                        className="sidebar-user"
                        onMouseEnter={() => handleNavHover(Settings)}
                        onFocus={() => handleNavHover(Settings)}
                      >
                        <div className="user-avatar">
                          {user.avatarUrl ? (
                            <img src={user.avatarUrl} alt="" />
                          ) : (
                            getInitials(user.displayName, user.email)
                          )}
                        </div>
                        <div className="user-details">
                          <span className="user-name">{user.displayName || user.email}</span>
                          <span className="user-role">{roleLabel}</span>
                        </div>
                      </Link>
                      <button
                        type="button"
                        className="btn sidebar-logout-btn"
                        onClick={() => {
                          void logout()
                          navigate('/auth/signin')
                        }}
                      >
                        <span className="sidebar-logout-btn__icon" aria-hidden="true">
                          🚪
                        </span>
                        {t('auth.logout')}
                      </button>
                    </>
                  )}
                  <div className="header-controls">
                    <LanguageSwitcher />
                    <ThemeToggle />
                  </div>
                  <div className="sidebar-footer__api">
                    <span className="status-dot" aria-hidden="true" />
                    <span>
                      {typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
                        ? t('common.local_api')
                        : `API · ${typeof window !== 'undefined' ? window.location.host : ''}/api`}
                    </span>
                  </div>
                </div>
              </aside>

              <main id="main-content" className="main" tabIndex={-1}>
                <header className="page-header">
                  <Breadcrumbs items={breadcrumbs} ariaLabel={t('common.primary_nav')} />
                  <p>{activeRoute?.eyebrow ?? t('common.not_found_eyebrow')}</p>
                  <div>
                    <h1>{activeRoute?.label ?? t('common.page_not_found')}</h1>
                    <span>{activeRoute?.description ?? t('common.not_found_desc')}</span>
                  </div>
                </header>

                <Suspense fallback={<LoadingScreen />}>
                  <Routes>
                    {routeDefs.map((route) => {
                      const Component = route.component
                      return (
                        <Route
                          key={route.path}
                          path={route.path}
                          element={
                            <ErrorBoundary key={route.path}>
                              {route.path === '/admin' ? (
                                <LocaleSection
                                  name="admin"
                                  fallback={<EmptyState description={t('common.module_loading')} />}
                                >
                                  <Component />
                                </LocaleSection>
                              ) : (
                                <Component />
                              )}
                            </ErrorBoundary>
                          }
                        />
                      )
                    })}
                    
                    <Route
                      path="/modeling/:modelId"
                      element={
                        <ErrorBoundary key="modeling-route-param">
                          <Modeling />
                        </ErrorBoundary>
                      }
                    />
                    <Route
                      path="/model/:modelId"
                      element={
                        <ErrorBoundary key="model-route-param">
                          <Modeling />
                        </ErrorBoundary>
                      }
                    />

                    <Route
                      path="*"
                      element={
                        <section className="card card--elevated">
                          <EmptyState title={t('common.module_not_found')} description={t('common.module_not_found_desc')}>
                            <a className="btn" href={DEFAULT_PATH} onClick={(event) => handleNavClick(event, DEFAULT_PATH)}>
                              {t('common.go_to_datasources')}
                            </a>
                          </EmptyState>
                        </section>
                      }
                    />
                  </Routes>
                </Suspense>
              </main>
            </div>
          </AuthGuard>
        }
      />
    </Routes>
  )
}

export default App
