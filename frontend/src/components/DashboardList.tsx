import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { useConfirm } from '../hooks/useConfirm'
import { EmptyState } from './ui/EmptyState'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { ErrorAlert } from './ui/ErrorAlert'

interface Dashboard {
  id: string
  name: string
  description?: string
  widgets: any[]
  created_at: string
}

interface DashboardListProps {
  onSelect: (id: string) => void
}

export default function DashboardList({ onSelect }: DashboardListProps) {
  const t = useT()
  const { get, postData, deleteData, loading, error } = useApi()
  const confirm = useConfirm()
  const [dashboards, setDashboards] = useState<Dashboard[]>([])
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const fetchDashboards = async () => {
    const data = await get<Dashboard[]>('/api/dashboards')
    if (data) {
      setDashboards(data)
    }
  }

  useEffect(() => {
    fetchDashboards()
  }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!name.trim()) {
      setFormError('Name is required')
      return
    }

    const payload = {
      name: name.trim(),
      description: description.trim() || undefined,
      widgets: [],
    }

    const res = await postData<Dashboard>('/api/dashboards', payload)
    if (res) {
      setIsModalOpen(false)
      setName('')
      setDescription('')
      fetchDashboards()
      onSelect(res.id)
    }
  }

  const handleDelete = async (e: React.MouseEvent, id: string, dashName: string) => {
    e.stopPropagation()
    const ok = await confirm({
      title: `Delete dashboard "${dashName}"?`,
      message: 'This action cannot be undone.',
      variant: 'danger',
    })
    if (!ok) return

    const res = await deleteData(`/api/dashboards/${id}`)
    if (res || error === null) {
      fetchDashboards()
    }
  }

  return (
    <div className="page-stack" style={{ position: 'relative' }}>
      <LoadingOverlay loading={loading} />

      {error && <ErrorAlert error={error} />}

      <div className="card">
        <div className="card-header-row card-header-row--spaced">
          <div>
            <h2>Custom Dashboards</h2>
            <p className="card-lead" style={{ marginTop: '0.4rem' }}>
              Create and organize custom dashboard layouts with charts, tables, and KPIs.
            </p>
          </div>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => setIsModalOpen(true)}
          >
            + New Dashboard
          </button>
        </div>
      </div>

      {dashboards.length === 0 ? (
        <div className="card" style={{ padding: '4rem 2rem', textAlign: 'center' }}>
          <EmptyState
            title="No Dashboards"
            description="Create your first custom dashboard to start building visual summaries."
          >
            <button
              type="button"
              className="btn btn-primary"
              style={{ marginTop: '1rem' }}
              onClick={() => setIsModalOpen(true)}
            >
              + Create Dashboard
            </button>
          </EmptyState>
        </div>
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
            gap: '1.5rem',
          }}
        >
          {dashboards.map((d) => (
            <div
              key={d.id}
              className="card card--elevated"
              style={{
                cursor: 'pointer',
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                minHeight: '160px',
                transition: 'transform 0.2s, box-shadow 0.2s',
              }}
              onClick={() => onSelect(d.id)}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-2px)'
                e.currentTarget.style.boxShadow = 'var(--shadow-lg)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'none'
                e.currentTarget.style.boxShadow = 'none'
              }}
            >
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <h3 style={{ margin: 0, fontSize: '1.2rem', fontWeight: 600 }}>{d.name}</h3>
                  <button
                    type="button"
                    style={{
                      background: 'none',
                      border: 'none',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                      fontSize: '1.1rem',
                      padding: '0 0.2rem',
                    }}
                    onClick={(e) => handleDelete(e, d.id, d.name)}
                    title="Delete Dashboard"
                  >
                    🗑️
                  </button>
                </div>
                {d.description && (
                  <p
                    style={{
                      color: 'var(--text-muted)',
                      fontSize: '0.9rem',
                      marginTop: '0.5rem',
                      lineHeight: '1.4',
                    }}
                  >
                    {d.description}
                  </p>
                )}
              </div>
              <div
                style={{
                  color: 'var(--text-muted)',
                  fontSize: '0.8rem',
                  marginTop: '1rem',
                  display: 'flex',
                  justifyContent: 'space-between',
                }}
              >
                <span>Widgets: {d.widgets?.length || 0}</span>
                <span>
                  {new Date(d.created_at).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric',
                  })}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}

      {isModalOpen && (
        <div className="modal-backdrop">
          <div className="modal-content" style={{ maxWidth: '480px' }}>
            <div className="modal-header">
              <h3>Create New Dashboard</h3>
              <button
                type="button"
                className="modal-close"
                onClick={() => setIsModalOpen(false)}
              >
                ✕
              </button>
            </div>
            <form onSubmit={handleCreate}>
              <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
                {formError && <div className="error-alert">{formError}</div>}
                <div className="form-field">
                  <label className="form-label" htmlFor="dash-name">
                    Name
                  </label>
                  <input
                    id="dash-name"
                    type="text"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="E.g., Sales Overview"
                    autoFocus
                    required
                  />
                </div>
                <div className="form-field">
                  <label className="form-label" htmlFor="dash-desc">
                    Description (optional)
                  </label>
                  <textarea
                    id="dash-desc"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="Brief summary of what this dashboard displays..."
                    rows={3}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setIsModalOpen(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Create Dashboard
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
