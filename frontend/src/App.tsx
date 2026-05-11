import { useEffect, useState, type ComponentType, type MouseEvent } from 'react'
import QueryBuilder from './components/QueryBuilder'
import Dashboard from './components/Dashboard'
import SavedQuestions from './components/SavedQuestions'
import AIQuery from './components/AIQuery'
import Datasources from './components/Datasources'
import Metadata from './components/Metadata'
import FewShotExamples from './components/FewShotExamples'
import Evaluation from './components/Evaluation'

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
    label: 'Veri Kaynakları',
    eyebrow: 'Bağlantılar',
    description: 'Veritabanı bağlayın, erişimi test edin ve metadata eşitleyin.',
    component: Datasources,
  },
  {
    path: '/metadata',
    label: 'Metadata',
    eyebrow: 'Katalog',
    description: 'Şemaları inceleyin, tablo açıklamalarını zenginleştirin ve AI bağlamını hazırlayın.',
    component: Metadata,
  },
  {
    path: '/query-builder',
    label: 'Sorgu Oluşturucu',
    eyebrow: 'Keşfet',
    description: 'Yönetilen mantıksal sorgular oluşturun ve üretilen SQL\'i önizleyin.',
    component: QueryBuilder,
  },
  {
    path: '/ai-query',
    label: 'AI Sorgu',
    eyebrow: 'Sor',
    description: 'Otomatik tablo yönlendirme ile doğal dilde soru sorun.',
    component: AIQuery,
  },
  {
    path: '/dashboard',
    label: 'Gösterge Paneli',
    eyebrow: 'Görselleştir',
    description: 'KPI örneklerini takip edin ve sorgu sonuçlarını grafikleyin.',
    component: Dashboard,
  },
  {
    path: '/saved',
    label: 'Kaydedilmiş Sorular',
    eyebrow: 'Kütüphane',
    description: 'Yeniden kullanılabilir soruları ve sorgu şablonlarını görüntüleyin.',
    component: SavedQuestions,
  },
  {
    path: '/few-shot-examples',
    label: 'Few-Shot Örnekleri',
    eyebrow: 'Yönetim',
    description: 'AI few-shot örneklerini yönetin ve text-to-SQL doğruluğunu artırın.',
    component: FewShotExamples,
  },
  {
    path: '/evaluation',
    label: 'Değerlendirme',
    eyebrow: 'Kalite',
    description: 'AI text-to-SQL değerlendirme sonuçlarını çalıştırın ve inceleyin.',
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
    document.title = activeRoute ? `${activeRoute.label} · Biqly` : 'Sayfa bulunamadı · Biqly'
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
            B
          </span>
          <span>
            <strong>Biqly</strong>
            <small>BI Çalışma Alanı</small>
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
          <ActiveComponent />
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
