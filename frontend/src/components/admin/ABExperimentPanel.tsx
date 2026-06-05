import '../../styles/ab-experiment.css'

import { useEffect, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { ABExperimentDetail } from './ABExperimentDetail'
import type { Experiment } from './ABExperimentForm'
import { ABExperimentForm } from './ABExperimentForm'
import { ABExperimentList } from './ABExperimentList'

type ViewState = 'list' | 'detail' | 'form'

function ABExperimentPanel() {
  const { get } = useAdminApi()
  const [view, setView] = useState<ViewState>('list')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [selectedExp, setSelectedExp] = useState<Experiment | null>(null)

  // Fetch the experiment detail if editing
  useEffect(() => {
    if (view === 'form' && selectedId && !selectedExp) {
      const fetchExp = async () => {
        const detail = await get<{ experiment: Experiment }>(`/api/ai/ab-experiments/${selectedId}`)
        if (detail) {
          setSelectedExp(detail.experiment)
        }
      }
      void fetchExp()
    }
  }, [view, selectedId, selectedExp, get])

  const handleSelect = (id: string) => {
    setSelectedId(id)
    setView('detail')
  }

  const handleCreate = () => {
    setSelectedId(null)
    setSelectedExp(null)
    setView('form')
  }

  const handleEdit = () => {
    setView('form')
  }

  const handleSave = (saved: Experiment) => {
    if (saved.id) {
      setSelectedId(saved.id)
      setSelectedExp(saved)
      setView('detail')
    } else {
      setView('list')
    }
  }

  const handleCancelForm = () => {
    if (selectedId) {
      setView('detail')
    } else {
      setView('list')
    }
  }

  const handleBackToList = () => {
    setSelectedId(null)
    setSelectedExp(null)
    setView('list')
  }

  return (
    <div className="ab-experiment-panel">
      {view === 'list' && <ABExperimentList onSelect={handleSelect} onCreate={handleCreate} />}
      {view === 'detail' && selectedId && (
        <ABExperimentDetail
          experimentId={selectedId}
          onBack={handleBackToList}
          onEdit={handleEdit}
        />
      )}
      {view === 'form' && (
        <ABExperimentForm
          experiment={selectedExp}
          onSave={handleSave}
          onCancel={handleCancelForm}
        />
      )}
    </div>
  )
}

export { ABExperimentPanel }
export default ABExperimentPanel
