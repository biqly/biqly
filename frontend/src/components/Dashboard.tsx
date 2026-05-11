import { useEffect, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell, LineChart, Line } from 'recharts'
import { useApi } from '../hooks/useApi'

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6']

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
          <div key={i} className="card" style={{ marginBottom: 0 }}>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>{card.label}</p>
            <p style={{ fontSize: '2rem', fontWeight: 700, margin: '0.5rem 0' }}>{card.value}</p>
            <p className="success" style={{ fontSize: '0.875rem' }}>{card.change}</p>
          </div>
        ))}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>Gelir eğilimi</h3>
          <div style={{ height: 300 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={sampleData.revenue}>
                <CartesianGrid strokeDasharray="3 3" stroke="#475569" />
                <XAxis dataKey="name" stroke="#94a3b8" />
                <YAxis stroke="#94a3b8" />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
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
                  {sampleData.countries.map((_: any, i: number) => (
                    <Cell key={i} fill={COLORS[i % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
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
                  {sampleData.orders.map((_: any, i: number) => (
                    <Cell key={i} fill={COLORS[(i + 3) % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
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
    </div>
  )
}

// ─── AI Usage Sub-component ─────────────────────────────────────────

interface AIUsageSummary {
  total_queries: number
  success_rate: number
  avg_latency_ms: number
  total_cost: number
}

interface DayUsage {
  date: string
  total_queries: number
  positive_feedback: number
  negative_feedback: number
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
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

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
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
        <div className="card" style={{ marginBottom: 0 }}>
          <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Toplam AI sorgusu</p>
          <p style={{ fontSize: '1.8rem', fontWeight: 700, margin: '0.3rem 0' }}>{summary.total_queries}</p>
        </div>
        <div className="card" style={{ marginBottom: 0 }}>
          <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Başarı oranı</p>
          <p style={{ fontSize: '1.8rem', fontWeight: 700, margin: '0.3rem 0' }}>{(summary.success_rate * 100).toFixed(0)}%</p>
        </div>
        <div className="card" style={{ marginBottom: 0 }}>
          <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Ort. gecikme</p>
          <p style={{ fontSize: '1.8rem', fontWeight: 700, margin: '0.3rem 0' }}>{summary.avg_latency_ms.toFixed(0)}ms</p>
        </div>
        <div className="card" style={{ marginBottom: 0 }}>
          <p style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Toplam maliyet</p>
          <p style={{ fontSize: '1.8rem', fontWeight: 700, margin: '0.3rem 0' }}>${summary.total_cost.toFixed(4)}</p>
        </div>
      </div>

      {/* Charts */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
        <div className="card">
          <h3>Günlük AI sorguları</h3>
          <div style={{ height: 250 }}>
            {trendData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={trendData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#475569" />
                  <XAxis dataKey="name" stroke="#94a3b8" />
                  <YAxis stroke="#94a3b8" />
                  <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
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
                  <CartesianGrid strokeDasharray="3 3" stroke="#475569" />
                  <XAxis dataKey="name" stroke="#94a3b8" />
                  <YAxis stroke="#94a3b8" />
                  <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
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
