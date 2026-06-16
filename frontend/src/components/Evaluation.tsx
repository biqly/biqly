import {
  type ComponentType,
  lazy,
  type LazyExoticComponent,
  startTransition,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react'
import { useNavigate } from 'react-router-dom'

import { useAdminApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import { localeLanguageTag, useI18n, useT } from '../i18n'
import { cn } from '../lib/cn'
import type { EvalRunDetail, EvalRunSummary, RegressionReport } from '../types/ai'
import type { EvalRunResponse } from './evaluation/demoData'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'

type PreloadableComponent<T extends ComponentType> = LazyExoticComponent<T> & {
  preload: () => Promise<{ default: T }>
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- React.lazy + named exports; props differ per tab
const lazyWithPreload = <T extends ComponentType<any>>(
  factory: () => Promise<{ default: T }>,
): PreloadableComponent<T> => {
  const Component = lazy(factory) as PreloadableComponent<T>
  Component.preload = factory
  return Component
}

const EvalRunTab = lazyWithPreload(() =>
  import('./evaluation/EvalRunTab').then((m) => ({ default: m.EvalRunTab })),
)
const EvalHistoryTab = lazyWithPreload(() =>
  import('./evaluation/EvalHistoryTab').then((m) => ({ default: m.EvalHistoryTab })),
)
const EvalRegressionTab = lazyWithPreload(() =>
  import('./evaluation/EvalRegressionTab').then((m) => ({ default: m.EvalRegressionTab })),
)

const TAB_COMPONENTS = {
  run: EvalRunTab,
  history: EvalHistoryTab,
  regression: EvalRegressionTab,
} as const

// DEMO_DATA is lazy-loaded from ./evaluation/demoData.ts

// ─── Main Component ────────────────────────────────────────────────

export default function Evaluation() {
  const navigate = useNavigate()
  const t = useT()
  const { locale } = useI18n()
  const localeTag = localeLanguageTag(locale)
  const adminApi = useAdminApi()

  const [evalData, setEvalData] = useState<EvalRunResponse | null>(null)
  const [running, setRunning] = useState(false)
  const [showDemo, setShowDemo] = useState(false)
  const [runError, setRunError] = useState<string | null>(null)

  // Eval runs history
  const [tabParam, setTabParam] = useQueryParam('tab')
  const initialTab: 'run' | 'history' | 'regression' =
    tabParam === 'history' || tabParam === 'regression' ? tabParam : 'run'
  const [activeTab, setActiveTab] = useState<'run' | 'history' | 'regression'>(initialTab)
  const [runHistory, setRunHistory] = useState<EvalRunSummary[]>([])
  const [selectedRun, setSelectedRun] = useState<EvalRunDetail | null>(null)
  const [historyLoaded, setHistoryLoaded] = useState(false)

  // Regression
  const [baselineParam, setBaselineParam] = useQueryParam('baseline')
  const [currentParam, setCurrentParam] = useQueryParam('current')
  const [baselineId, setBaselineId] = useState(baselineParam)
  const [currentId, setCurrentId] = useState(currentParam)
  const [regression, setRegression] = useState<RegressionReport | null>(null)
  const [regressionLoading, setRegressionLoading] = useState(false)

  const handleTabHover = (hoveredTab: 'run' | 'history' | 'regression') => {
    const comp = TAB_COMPONENTS[hoveredTab]
    if (typeof comp.preload === 'function') {
      void comp.preload()
    }
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      Object.values(TAB_COMPONENTS).forEach((comp) => {
        if (typeof comp.preload === 'function') {
          void comp.preload()
        }
      })
    }, 1500)
    return () => clearTimeout(timer)
  }, [])

  useEffect(() => {
    setTabParam(activeTab === 'run' ? '' : activeTab)
  }, [activeTab, setTabParam])
  useEffect(() => {
    setBaselineParam(baselineId)
  }, [baselineId, setBaselineParam])
  useEffect(() => {
    setCurrentParam(currentId)
  }, [currentId, setCurrentParam])

  const loadRunHistory = useCallback(async () => {
    if (!adminApi.configured || historyLoaded) {
      return
    }
    const data = await adminApi.get<EvalRunSummary[]>('/api/ai/eval/runs')
    if (data) {
      setRunHistory(data)
    }
    setHistoryLoaded(true)
  }, [adminApi, historyLoaded])

  useEffect(() => {
    if (activeTab === 'history' || activeTab === 'regression') {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      void loadRunHistory()
    }
  }, [activeTab, loadRunHistory])

  useEffect(() => {
    if (selectedRun) {
      return
    } // don't reload if already selected
    // Auto-select latest run when on history tab
    if (activeTab === 'history' && runHistory.length > 0) {
      // Show list, don't auto-select
    }
  }, [activeTab, runHistory, selectedRun])

  const loadRunDetail = async (runId: string) => {
    if (!adminApi.configured) {
      return
    }
    const data = await adminApi.get<EvalRunDetail>(`/api/ai/eval/runs/${runId}`)
    if (data) {
      setSelectedRun(data)
    }
  }

  const runRegression = async () => {
    if (!baselineId || !currentId || !adminApi.configured) {
      return
    }
    setRegressionLoading(true)
    setRegression(null)
    const data = await adminApi.get<RegressionReport>(
      `/api/ai/eval/regression?baseline=${baselineId}&current=${currentId}`,
    )
    if (data) {
      setRegression(data)
    }
    setRegressionLoading(false)
  }

  // Derive pie chart data from current eval data
  const pieData = useMemo(() => {
    if (!evalData) {
      return []
    }
    return [
      { name: t('evaluation.pie_passed'), value: evalData.passed, fill: '#22c55e' },
      { name: t('evaluation.pie_failed'), value: evalData.failed, fill: '#ef4444' },
    ]
  }, [evalData, t])

  // Derive trend data
  const trendData = useMemo(() => {
    if (!evalData?.accuracy_trend) {
      return []
    }
    return evalData.accuracy_trend.map((d) => ({
      ...d,
      pass_rate_pct: Math.round(d.pass_rate * 100),
    }))
  }, [evalData])

  const runEvaluation = async () => {
    if (!adminApi.configured) {
      setRunError(t('evaluation.admin_key_missing_run'))
      return
    }
    setRunning(true)
    setRunError(null)
    setShowDemo(false)
    try {
      const res = await adminApi.postData<EvalRunResponse>('/api/ai/eval/run', {})
      if (res) {
        setEvalData(res)
      } else {
        setEvalData(null)
      }
    } catch {
      setShowDemo(true)
      void import('./evaluation/demoData').then((m) => setEvalData(m.DEMO_DATA))
    } finally {
      setRunning(false)
    }
  }

  const activeData = evalData

  return (
    <div className="grid gap-4">
      {/* Tabs */}
      <div
        className={`border-border mb-2 inline-flex w-fit items-center gap-1 rounded-lg border bg-[rgba(255,255,255,0.02)] p-1`}
        role="tablist"
        aria-label={t('evaluation.tabs_aria')}
      >
        {[
          { key: 'run' as const, label: t('evaluation.tab_run') },
          {
            key: 'history' as const,
            label: t('evaluation.tab_history', { count: runHistory.length }),
          },
          { key: 'regression' as const, label: t('evaluation.tab_regression') },
        ].map((tab) => {
          const isActive = activeTab === tab.key
          return (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              className={cn(
                'mt-0 w-auto rounded-[0.35rem] border px-4 py-1.5 text-[0.78rem] font-semibold shadow-none transition-all duration-180 ease-out',
                isActive
                  ? 'bg-card-raised border-border-strong text-foreground shadow-[0_2px_8px_rgba(0,0,0,0.15)]'
                  : 'text-foreground-muted hover:text-foreground border-transparent bg-transparent hover:border-transparent hover:bg-[rgba(255,255,255,0.04)]',
              )}
              onClick={() => {
                startTransition(() => {
                  setActiveTab(tab.key)
                  setSelectedRun(null)
                  setRegression(null)
                })
              }}
              onMouseEnter={() => handleTabHover(tab.key)}
              onFocus={() => handleTabHover(tab.key)}
            >
              {tab.label}
            </button>
          )
        })}
      </div>

      {!adminApi.configured && (
        <ErrorAlert error={t('evaluation.admin_key_missing_ui')}>
          {' '}
          <a
            href="/settings"
            onClick={(e) => {
              e.preventDefault()
              void navigate('/settings')
            }}
          >
            {t('evaluation.admin_key_open_settings')}
          </a>
        </ErrorAlert>
      )}

      <Suspense fallback={<LoadingScreen minHeight="200px" />}>
        {/* ─── TAB: Run Evaluation ─────────────────────────────── */}
        {activeTab === 'run' && (
          <EvalRunTab
            running={running}
            runEvaluation={() => {
              void runEvaluation()
            }}
            showDemo={showDemo}
            runError={runError}
            activeData={activeData}
            pieData={pieData}
            trendData={trendData}
            t={t}
          />
        )}

        {/* ─── TAB: History ────────────────────────────────────── */}
        {activeTab === 'history' && (
          <EvalHistoryTab
            runHistory={runHistory}
            selectedRun={selectedRun}
            setSelectedRun={setSelectedRun}
            loadRunDetail={(runId) => {
              void loadRunDetail(runId)
            }}
            localeTag={localeTag}
            t={t}
          />
        )}

        {/* ─── TAB: Regression ─────────────────────────────────── */}
        {activeTab === 'regression' && (
          <EvalRegressionTab
            runHistory={runHistory}
            baselineId={baselineId}
            currentId={currentId}
            setBaselineId={setBaselineId}
            setCurrentId={setCurrentId}
            regression={regression}
            regressionLoading={regressionLoading}
            runRegression={() => {
              void runRegression()
            }}
            configured={adminApi.configured}
            localeTag={localeTag}
            t={t}
          />
        )}
      </Suspense>
    </div>
  )
}
