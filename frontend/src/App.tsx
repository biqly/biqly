import { Suspense, lazy, useEffect, useState, type ComponentType, type LazyExoticComponent, type MouseEvent, type ReactNode } from 'react'
import { ErrorBoundary } from './components/ui/ErrorBoundary'

const Datasources = lazy(() => import('./components/Datasources'))
const Metadata = lazy(() => import('./components/Metadata'))
const QueryBuilder = lazy(() => import('./components/QueryBuilder'))
const AIQuery = lazy(() => import('./components/AIQuery'))
const SavedQuestions = lazy(() => import('./components/SavedQuestions'))
const FewShotExamples = lazy(() => import('./components/FewShotExamples'))
const Evaluation = lazy(() => import('./components/Evaluation'))

interface AppRoute {
  path: string
  label: string
  eyebrow: string
  description: string
  icon: ReactNode
  component: LazyExoticComponent<ComponentType>
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

const routes: AppRoute[] = [
  {
    path: '/datasources',
    label: 'Veri Kaynakları',
    eyebrow: 'Bağlantılar',
    description: 'Veritabanı bağlayın, erişimi test edin ve metadata eşitleyin.',
    icon: IconDatasources,
    component: Datasources,
  },
  {
    path: '/metadata',
    label: 'Metadata',
    eyebrow: 'Katalog',
    description: 'Şemaları inceleyin, tablo açıklamalarını zenginleştirin ve AI bağlamını hazırlayın.',
    icon: IconMetadata,
    component: Metadata,
  },
  {
    path: '/query-builder',
    label: 'Sorgu Oluşturucu',
    eyebrow: 'Keşfet',
    description: 'Yönetilen mantıksal sorgular oluşturun ve üretilen SQL\'i önizleyin.',
    icon: IconQueryBuilder,
    component: QueryBuilder,
  },
  {
    path: '/ai-query',
    label: 'AI Sorgu',
    eyebrow: 'Sor',
    description: 'Otomatik tablo yönlendirme ile doğal dilde soru sorun.',
    icon: IconAIQuery,
    component: AIQuery,
  },
  {
    path: '/saved',
    label: 'Kaydedilmiş Sorular',
    eyebrow: 'Kütüphane',
    description: 'Yeniden kullanılabilir soruları ve sorgu şablonlarını görüntüleyin.',
    icon: IconSaved,
    component: SavedQuestions,
  },
  {
    path: '/few-shot-examples',
    label: 'Few-Shot Örnekleri',
    eyebrow: 'Yönetim',
    description: 'AI few-shot örneklerini yönetin ve text-to-SQL doğruluğunu artırın.',
    icon: IconFewShot,
    component: FewShotExamples,
  },
  {
    path: '/evaluation',
    label: 'Değerlendirme',
    eyebrow: 'Kalite',
    description: 'AI text-to-SQL değerlendirme sonuçlarını çalıştırın ve inceleyin.',
    icon: IconEvaluation,
    component: Evaluation,
  },
]

const defaultRoute = routes[0]!

const findRoute = (pathname: string) => routes.find((route) => route.path === pathname)

const initialPath = () => {
  const { pathname } = window.location
  if (pathname === '/') return defaultRoute.path
  return findRoute(pathname)?.path ?? pathname
}

function App() {
  const [activePath, setActivePath] = useState(initialPath)
  const activeRoute = findRoute(activePath)
  const ActiveComponent = activeRoute?.component

  useEffect(() => {
    if (window.location.pathname === '/') {
      window.history.replaceState(null, '', defaultRoute.path)
    }
  }, [])

  useEffect(() => {
    const handlePopState = () => setActivePath(initialPath())
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    document.title = activeRoute ? `${activeRoute.label} · ABI` : 'Sayfa bulunamadı · ABI'
  }, [activeRoute])

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
        İçeriğe atla
      </a>

      <aside className="sidebar" aria-label="Ana gezinme">
        <a className="brand" href={defaultRoute.path} onClick={(event) => handleNavClick(event, defaultRoute.path)}>
          <span className="brand-mark" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round">
              <path d="M4.9 9 Q8.05 7.1 11.2 9" strokeWidth="1.5" />
              <path d="M12.8 9 Q15.95 7.1 19.1 9" strokeWidth="1.5" />
              <circle cx="8" cy="12" r="0.95" fill="currentColor" stroke="none" />
              <circle cx="16" cy="12" r="0.95" fill="currentColor" stroke="none" />
              <path d="M6 15.4 C 7.4 18.55 10.5 18.95 12 16.5 C 13.5 18.95 16.6 18.55 18 15.4" strokeWidth="1.7" />
            </svg>
          </span>
          <span className="brand-text">
            <strong>ABI</strong>
            <small>Artificial Business Intelligence</small>
          </span>
        </a>

        <nav className="nav">
          {routes.map((route) => (
            <a
              key={route.path}
              className="nav-link"
              href={route.path}
              aria-current={activeRoute?.path === route.path ? 'page' : undefined}
              onClick={(event) => handleNavClick(event, route.path)}
            >
              <span className="nav-icon" aria-hidden="true">{route.icon}</span>
              <span className="nav-label">{route.label}</span>
              <small>{route.eyebrow}</small>
            </a>
          ))}
        </nav>

        <div className="sidebar-footer">
          <span className="status-dot" aria-hidden="true" />
          <span>Yerel API · localhost:8888</span>
        </div>
      </aside>

      <main id="main-content" className="main" tabIndex={-1}>
        <header className="page-header">
          <p>{activeRoute?.eyebrow ?? 'Bulunamadı'}</p>
          <div>
            <h1>{activeRoute?.label ?? 'Sayfa Bulunamadı'}</h1>
            <span>{activeRoute?.description ?? 'Bu modül mevcut değil veya bağlantı güncel değil.'}</span>
          </div>
        </header>

        {ActiveComponent ? (
          <ErrorBoundary key={activeRoute.path}>
            <Suspense fallback={<section className="card empty-state"><h2>Modül yükleniyor</h2></section>}>
              <ActiveComponent />
            </Suspense>
          </ErrorBoundary>
        ) : (
          <section className="card empty-state">
            <h2>Modül Bulunamadı</h2>
            <p>İstenen sayfa mevcut değil. Var olan bir modülü açmak için navigasyonu kullanın.</p>
            <a className="btn" href={defaultRoute.path} onClick={(event) => handleNavClick(event, defaultRoute.path)}>
              Veri Kaynaklarına Git
            </a>
          </section>
        )}
      </main>
    </div>
  )
}

export default App
