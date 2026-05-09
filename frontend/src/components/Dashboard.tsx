import { useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6']

// Demo dashboard with sample data
export default function Dashboard() {
  const [selectedRange, setSelectedRange] = useState('7d')

  const sampleData = {
    revenue: [
      { name: 'Mon', value: 4200 },
      { name: 'Tue', value: 3800 },
      { name: 'Wed', value: 5100 },
      { name: 'Thu', value: 4600 },
      { name: 'Fri', value: 6200 },
      { name: 'Sat', value: 3100 },
      { name: 'Sun', value: 2800 },
    ],
    countries: [
      { name: 'USA', value: 45 },
      { name: 'UK', value: 25 },
      { name: 'Germany', value: 15 },
      { name: 'France', value: 10 },
      { name: 'Japan', value: 5 },
    ],
    orders: [
      { name: 'Completed', value: 340 },
      { name: 'Pending', value: 120 },
      { name: 'Cancelled', value: 45 },
    ],
  }

  return (
    <div>
      <div className="card">
        <h2>Dashboard</h2>
        <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
          {['24h', '7d', '30d', '90d'].map((r) => (
            <button
              key={r}
              className={selectedRange === r ? 'btn' : ''}
              style={{ background: selectedRange === r ? undefined : 'var(--bg-card)', color: selectedRange === r ? undefined : 'var(--text-secondary)' }}
              onClick={() => setSelectedRange(r)}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '1.5rem', marginBottom: '1.5rem' }}>
        {[
          { label: 'Total Revenue', value: '$29,800', change: '+12.5%' },
          { label: 'Orders', value: '505', change: '+8.2%' },
          { label: 'Avg Order Value', value: '$59.01', change: '+3.1%' },
          { label: 'Active Users', value: '1,247', change: '+15.3%' },
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
          <h3>Revenue Trend</h3>
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
          <h3>Orders by Country</h3>
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
          <h3>Order Status</h3>
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
          <h3>Recent Queries</h3>
          <table className="results-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Model</th>
                <th>Rows</th>
                <th>Duration</th>
              </tr>
            </thead>
            <tbody>
              {[
                { time: '2 min ago', model: 'orders', rows: 12, ms: 45 },
                { time: '5 min ago', model: 'users', rows: 8, ms: 23 },
                { time: '12 min ago', model: 'products', rows: 25, ms: 67 },
                { time: '18 min ago', model: 'orders', rows: 3, ms: 12 },
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
    </div>
  )
}
