import { useEffect, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell, LineChart, Line } from 'recharts'
import { useApi } from '../hooks/useApi'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { chartColor } from '../utils/constants'
import { getRateColor } from '../utils/formatters'
import { KPICard } from './ui/KPICard'
import type { ModelStats } from '../types/ai'

// Demo dashboard with sample data
export default function Dashboard() {
  const [selectedRange, setSelectedRange] = useState('7d')

  const sampleData = {
    revenue: [
      { name: 'Pzt', value: 4200 },
      { name: 'Sal', value: 3800 },
      { name: 'Çar', value: 5100 },
      { name: 'Per', value: 4600 },
      { name: 'Cum', value: 6200 },
      { name: 'Cts', value: 3100 },
      { name: 'Paz', value: 2800 },
    ],
    countries: [
      { name: 'USA', value: 45 },
      { name: 'UK', value: 25 },
      { name: 'Germany', value: 15 },
      { name: 'France', value: 10 },
      { name: 'Japan', value: 5 },
    ],
    orders: [
      { name: 'Tamamlandı', value: 340 },
      { name: 'Beklemede', value: 120 },
      { name: 'İptal', value: 45 },
    ],
  }

  return (
    <div>
      <div className="card">
        <h2>Raporlama aralığı</h2>
        <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
          {[
            { key: '24h', label: '24 saat' },
            { key: '7d', label: '7 gün' },
            { key: '30d', label: '30 gün' },
            { key: '90d', label: '90 gün' },
          ].map(({ key, label }) => (
            <button
              key={key}
              className={selectedRange === key ? 'btn' : ''}
              style={{ background: selectedRange === key ? undefined : 'var(--bg-card)', color: selectedRange === key ? undefined : 'var(--text-secondary)' }}
              onClick={() => setSelectedRange(key)}
              aria-pressed={selectedRange === key}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '1.5rem', marginBottom: '1.5rem' }}>
        {[
          { label: 'Toplam gelir', value: '$29,800', change: '+%12,5' },
          { label: 'Siparişler', value: '505', change: '+%8,2' },
          { label: 'Ort. sipariş tutarı', value: '$59,01', change: '+%3,1' },
          { label: 'Aktif kullanıcılar', value: '1.247', change: '+%15,3' },
        ].map((card, i) => (
          <KPICard key={i} label={`${card.label} ${card.change}`} value={card.value} color="var(--success)" />
        ))}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>Gelir eğilimi</h3>
          <div style={{ height: 300 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={sampleData.revenue}>
                <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
                <XAxis dataKey="name" stroke={chartAxisStroke} />
                <YAxis stroke={chartAxisStroke} />
                <Tooltip contentStyle={chartTooltipStyle} />
                <Bar dataKey="value" fill="#3b82f6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="card">
          <h3>Ülkelere göre siparişler</h3>
          <div style={{ height: 300 }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={sampleData.countries} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>
                  {sampleData.countries.map((_, i) => (
                    <Cell key={i} fill={chartColor(i)} />
                  ))}
                </Pie>
                <Tooltip contentStyle={chartTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="card">
          <h3>Sipariş durumu</h3>
          <div style={{ height: 300 }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={sampleData.orders} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>
                  {sampleData.orders.map((_, i) => (
                    <Cell key={i} fill={chartColor(i + 3)} />
                  ))}
                </Pie>
                <Tooltip contentStyle={chartTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="card">
          <h3>Son sorgular</h3>
          <table className="results-table">
            <thead>
              <tr>
                <th>Zaman</th>
                <th>Model</th>
                <th>Satır</th>
                <th>Süre</th>
              </tr>
            </thead>
            <tbody>
              {[
                { time: '2 dk önce', model: 'orders', rows: 12, ms: 45 },
                { time: '5 dk önce', model: 'users', rows: 8, ms: 23 },
                { time: '12 dk önce', model: 'products', rows: 25, ms: 67 },
                { time: '18 dk önce', model: 'orders', rows: 3, ms: 12 },
              ].map((q, i) => (
                <tr key={i}>
                  <td>{q.time}</td>
                  <td>{q.model}</td>
                  <td>{q.rows}</td>
                  <td>{q.ms}ms</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* ─── AI Usage Section ─────────────────────────────────── */}
      <AIUsageSection />

      {/* ─── Model Success Rates ──────────────────────────────── */}
      <ModelSuccessRates />
    </div>
  )
}

// ─── AI Usage Sub-component ─────────────────────────────────────────

interface AIUsageSummary {
  total_queries: number
  success_rate: number
  failure_rate: number
  avg_retry_count: number
  avg_latency_ms: number
  total_cost: number
  positive_feedback?: number
  negative_feedback?: number
}

interface DayUsage {
  date: string
  total_queries: number
  failure_rate: number
  avg_retry_count: number
  avg_latency_ms: number
  total_cost: number
  total_tokens: number
}

function AIUsageSection() {
  const { get } = useApi()
  const [summary, setSummary] = useState<AIUsageSummary | null>(null)
  const [daily, setDaily] = useState<DayUsage[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    get<{ summary: AIUsageSummary; daily: DayUsage[] }>('/api/ai/usage').then((data) => {
      if (data) {
        setSummary(data.summary)
        setDaily(data.daily.slice(0, 10).reverse()) // Show last 10 days ascending
      }
      setLoading(false)
    })
  }, [])

  if (loading) return null
  if (!summary) return null

  // Top questions by frequency — from daily data, derive top questions
  // In production, backend would return this; for now we show the trend
  const trendData = daily.map((d) => ({
    name: d.date.slice(5), // MM-DD
    queries: d.total_queries,
    cost: parseFloat(d.total_cost.toFixed(3)),
  }))

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ marginBottom: '1rem' }}>🤖 AI kullanımı (son 30 gün)</h2>

      {/* KPI Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
        <KPICard label="Toplam AI sorgusu" value={summary.total_queries} color="var(--accent)" />
        <KPICard label="Başarı oranı" value={`${(summary.success_rate * 100).toFixed(0)}%`} color={getRateColor(summary.success_rate * 100)} />
        <KPICard label="Hata oranı" value={`${(summary.failure_rate * 100).toFixed(0)}%`} color={getRateColor(100 - summary.failure_rate * 100)} />
        <KPICard label="Ort. yeniden deneme" value={summary.avg_retry_count.toFixed(2)} color="var(--text-muted)" />
        <KPICard label="Ort. gecikme" value={`${summary.avg_latency_ms.toFixed(0)}ms`} color="var(--warning)" />
        <KPICard label="Toplam maliyet" value={`$${summary.total_cost.toFixed(4)}`} color="var(--success)" />
      </div>

      {/* Charts */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>Günlük AI sorguları</h3>
          <div style={{ height: 250 }}>
            {trendData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={trendData}>
                  <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
                  <XAxis dataKey="name" stroke={chartAxisStroke} />
                  <YAxis stroke={chartAxisStroke} />
                  <Tooltip contentStyle={chartTooltipStyle} />
                  <Line type="monotone" dataKey="queries" stroke="#3b82f6" strokeWidth={2} />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>Henüz AI sorgusu yok</p>
            )}
          </div>
        </div>

        <div className="card">
          <h3>Günlük maliyet</h3>
          <div style={{ height: 250 }}>
            {trendData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={trendData}>
                  <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
                  <XAxis dataKey="name" stroke={chartAxisStroke} />
                  <YAxis stroke={chartAxisStroke} />
                  <Tooltip contentStyle={chartTooltipStyle} />
                  <Bar dataKey="cost" fill="#f59e0b" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <p style={{ color: 'var(--text-muted)', textAlign: 'center', paddingTop: '4rem' }}>Henüz maliyet verisi yok</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── Model Success Rates Sub-component ──────────────────────────────

function ModelSuccessRates() {
  const { get } = useApi()
  const [models, setModels] = useState<ModelStats[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    get<ModelStats[]>('/api/ai/stats/models').then((data) => {
      if (data) setModels(data)
      setLoading(false)
    })
  }, [])

  if (loading) return null
  if (models.length === 0) return null

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ marginBottom: '1rem' }}>📊 Model bazlı başarı oranları</h2>
      <table className="results-table">
        <thead>
          <tr>
            <th>Model</th>
            <th style={{ textAlign: 'right' }}>Toplam</th>
            <th style={{ textAlign: 'right' }}>Başarılı</th>
            <th style={{ textAlign: 'right' }}>Başarısız</th>
            <th style={{ textAlign: 'right' }}>Başarı %</th>
            <th style={{ textAlign: 'right' }}>Güven</th>
            <th style={{ textAlign: 'right' }}>Gecikme</th>
            <th style={{ textAlign: 'right' }}>👍</th>
            <th style={{ textAlign: 'right' }}>👎</th>
          </tr>
        </thead>
        <tbody>
          {models.map((m) => (
            <tr key={m.model_id}>
              <td>{m.model_name || m.model_id}</td>
              <td style={{ textAlign: 'right' }}>{m.total_queries}</td>
              <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.success_count}</td>
              <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.failure_count}</td>
              <td style={{ textAlign: 'right' }}>
                <span style={{
                  color: getRateColor(m.success_rate),
                  fontWeight: 700,
                }}>
                  {m.success_rate.toFixed(1)}%
                </span>
              </td>
              <td style={{ textAlign: 'right' }}>{(m.avg_confidence * 100).toFixed(0)}%</td>
              <td style={{ textAlign: 'right' }}>{m.avg_latency_ms.toFixed(0)}ms</td>
              <td style={{ textAlign: 'right', color: 'var(--success)' }}>{m.positive_count}</td>
              <td style={{ textAlign: 'right', color: 'var(--error)' }}>{m.negative_count}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* Bar chart visualization */}
      <div className="card" style={{ marginTop: '1rem' }}>
        <h3>Başarı oranı karşılaştırması</h3>
        <div style={{ height: 250 }}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={models.map((m) => ({
              name: m.model_name || m.model_id,
              success_rate: m.success_rate,
              confidence: m.avg_confidence * 100,
            }))}>
              <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
              <XAxis dataKey="name" stroke={chartAxisStroke} tick={{ fontSize: 11 }} />
              <YAxis stroke={chartAxisStroke} domain={[0, 100]} />
              <Tooltip contentStyle={chartTooltipStyle} />
              <Bar dataKey="success_rate" fill="#22c55e" radius={[4, 4, 0, 0]} name="Başarı %" />
              <Bar dataKey="confidence" fill="#3b82f6" radius={[4, 4, 0, 0]} name="Güven %" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}
