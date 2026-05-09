import { useEffect, useState, type ComponentType, type MouseEvent } from 'react'
import QueryBuilder from './components/QueryBuilder'
import Dashboard from './components/Dashboard'
import SavedQuestions from './components/SavedQuestions'
import AIQuery from './components/AIQuery'
import Datasources from './components/Datasources'
import Metadata from './components/Metadata'

interface AppRoute {
  path: string
  label: string
  eyebrow: string
  description: string
  component: ComponentType
}

const routes: AppRoute[] = [
  {
    path: '/datasources',
    label: 'Datasources',
    eyebrow: 'Connections',
    description: 'Connect databases, test access, and sync metadata.',
    component: Datasources,
  },
  {
    path: '/metadata',
    label: 'Metadata',
    eyebrow: 'Catalog',
    description: 'Review schemas, enrich table descriptions, and prepare AI context.',
    component: Metadata,
  },
  {
    path: '/query-builder',
    label: 'Query Builder',
    eyebrow: 'Explore',
    description: 'Build governed logical queries and preview generated SQL.',
    component: QueryBuilder,
  },
  {
    path: '/ai-query',
    label: 'AI Query',
    eyebrow: 'Ask',
    description: 'Ask natural-language questions with automatic table routing.',
    component: AIQuery,
  },
  {
    path: '/dashboard',
    label: 'Dashboard',
    eyebrow: 'Visualize',
    description: 'Track KPI examples and chart query results.',
    component: Dashboard,
  },
  {
    path: '/saved',
    label: 'Saved Questions',
    eyebrow: 'Library',
    description: 'Browse reusable questions and query templates.',
    component: SavedQuestions,
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
    document.title = activeRoute ? `${activeRoute.label} · Biqly` : 'Page Not Found · Biqly'
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
        Skip to content
      </a>

      <aside className="sidebar" aria-label="Primary navigation">
        <a className="brand" href={defaultRoute.path} onClick={(event) => handleNavClick(event, defaultRoute.path)}>
          <span className="brand-mark" aria-hidden="true">
            B
          </span>
          <span>
            <strong>Biqly</strong>
            <small>BI Workspace</small>
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
              <span>{route.label}</span>
              <small>{route.eyebrow}</small>
            </a>
          ))}
        </nav>

        <div className="sidebar-footer">
          <span className="status-dot" aria-hidden="true" />
          <span>Local API · localhost:8888</span>
        </div>
      </aside>

      <main id="main-content" className="main" tabIndex={-1}>
        <header className="page-header">
          <p>{activeRoute?.eyebrow ?? 'Not Found'}</p>
          <div>
            <h1>{activeRoute?.label ?? 'Page Not Found'}</h1>
            <span>{activeRoute?.description ?? 'This module does not exist or the link is outdated.'}</span>
          </div>
        </header>

        {ActiveComponent ? (
          <ActiveComponent />
        ) : (
          <section className="card empty-state">
            <h2>Module Not Found</h2>
            <p>The requested page is not available. Use the navigation to open an existing module.</p>
            <a className="btn" href={defaultRoute.path} onClick={(event) => handleNavClick(event, defaultRoute.path)}>
              Go to Datasources
            </a>
          </section>
        )}
      </main>
    </div>
  )
}

export default App
