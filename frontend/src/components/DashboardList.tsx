import { useCallback, useEffect, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useAutofocus } from '../hooks/useAutofocus'
import { useConfirm } from '../hooks/useConfirm'
import { useT } from '../i18n'
import { adminBtnAutoWidthClass } from './admin/adminClasses'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
import { Modal } from './ui/Modal'

interface Dashboard {
  id: string
  name: string
  description?: string
  widgets: unknown[]
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
  const createNameInputRef = useAutofocus<HTMLInputElement>(isModalOpen)

  const fetchDashboards = useCallback(async () => {
    const data = await get<Dashboard[]>('/api/dashboards')
    if (data) {
      setDashboards(data)
    }
  }, [get])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void fetchDashboards()
  }, [fetchDashboards])

  const closeCreateModal = () => {
    setIsModalOpen(false)
    setName('')
    setDescription('')
    setFormError(null)
  }

  const openCreateModal = () => {
    setName('')
    setDescription('')
    setFormError(null)
    setIsModalOpen(true)
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!name.trim()) {
      setFormError(t('customDashboards.name_required'))
      return
    }

    const payload = {
      name: name.trim(),
      description: description.trim() || undefined,
      widgets: [],
    }

    const res = await postData<Dashboard>('/api/dashboards', payload)
    if (res) {
      closeCreateModal()
      void fetchDashboards()
      onSelect(res.id)
    }
  }

  const handleDelete = async (e: React.MouseEvent, id: string, dashName: string) => {
    e.stopPropagation()
    const ok = await confirm({
      title: t('customDashboards.delete_title', { name: dashName }),
      message: t('customDashboards.delete_message'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }

    const res = await deleteData(`/api/dashboards/${id}`)
    if (res || error === null) {
      void fetchDashboards()
    }
  }

  if (loading && dashboards.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className="page-stack dashboard-list-page" style={{ position: 'relative' }}>
      <LoadingOverlay loading={loading}>
        {error && <ErrorAlert error={error} />}

        <div className="card">
          <div className="card-header-row card-header-row--spaced">
            <div>
              <h2>{t('customDashboards.title')}</h2>
              <p className="card-lead" style={{ marginTop: '0.4rem' }}>
                {t('customDashboards.lead')}
              </p>
            </div>
            <button type="button" className="btn btn-primary" onClick={openCreateModal}>
              + {t('customDashboards.new')}
            </button>
          </div>
        </div>

        {dashboards.length === 0 ? (
          <div className="card py-16 px-8 text-center">
            <EmptyState
              title={t('customDashboards.empty_title')}
              description={t('customDashboards.empty_description')}
            >
              <button
                type="button"
                className={`btn btn-primary ${adminBtnAutoWidthClass} mt-4`}
                onClick={openCreateModal}
              >
                + {t('customDashboards.empty_cta')}
              </button>
            </EmptyState>
          </div>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-6">
            {dashboards.map((d) => (
              <div
                key={d.id}
                className="card card--elevated flex flex-col justify-between min-h-[10rem] cursor-pointer transition-all duration-200 ease-out hover:-translate-y-0.5 hover:shadow-lg"
                role="button"
                tabIndex={0}
                onClick={() => onSelect(d.id)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    onSelect(d.id)
                  }
                }}
              >
                <div>
                  <div className="flex justify-between items-start gap-2">
                    <h3 className="m-0 text-[1.2rem] font-semibold">{d.name}</h3>
                    <button
                      type="button"
                      className="shrink-0 px-[0.2rem] py-0 border-0 bg-transparent text-foreground-faint text-[1.1rem] leading-none cursor-pointer hover:text-foreground"
                      onClick={(e) => {
                        void handleDelete(e, d.id, d.name)
                      }}
                      title={t('customDashboards.delete_tooltip')}
                      aria-label={t('customDashboards.delete_tooltip')}
                    >
                      🗑️
                    </button>
                  </div>
                  {d.description && (
                    <p className="mt-2 mb-0 mx-0 text-foreground-faint text-[0.9rem] leading-[1.4]">
                      {d.description}
                    </p>
                  )}
                </div>
                <div className="flex justify-between gap-3 mt-4 text-foreground-faint text-[0.8rem]">
                  <span>{t('customDashboards.widgets_count', { count: d.widgets.length })}</span>
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
      </LoadingOverlay>

      <Modal
        open={isModalOpen}
        title={t('customDashboards.create_title')}
        subtitle={t('customDashboards.create_subtitle')}
        onClose={closeCreateModal}
        className="modal-card--dashboard"
        labelledBy="dashboard-create-title"
      >
        <form
          onSubmit={(e) => {
            void handleCreate(e)
          }}
          className="dashboard-create-form"
        >
          {formError && <ErrorAlert error={formError} />}
          <div className="form-group">
            <label htmlFor="dash-name">{t('customDashboards.name_label')}</label>
            <input
              id="dash-name"
              type="text"
              ref={createNameInputRef}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t('customDashboards.name_placeholder')}
              required
            />
          </div>
          <div className="form-group">
            <label htmlFor="dash-desc">
              {t('customDashboards.desc_label')}{' '}
              <span className="font-normal">({t('common.optional')})</span>
            </label>
            <textarea
              id="dash-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('customDashboards.desc_placeholder')}
              rows={3}
            />
          </div>
          <div className="modal-actions">
            <button
              type="button"
              className={`btn btn-secondary ${adminBtnAutoWidthClass}`}
              onClick={closeCreateModal}
            >
              {t('customDashboards.cancel')}
            </button>
            <button
              type="submit"
              className={`btn btn-primary ${adminBtnAutoWidthClass}`}
              disabled={loading || !name.trim()}
            >
              {t('customDashboards.create')}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  )
}
