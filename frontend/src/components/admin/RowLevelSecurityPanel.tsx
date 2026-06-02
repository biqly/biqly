import { useEffect, useMemo, useState } from 'react'
import { useT } from '../../i18n'
import { useDatasources } from '../../hooks/useDatasources'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useModelDetail } from '../../hooks/useModelDetail'
import { getSecurityPolicyByKeys, upsertSecurityPolicy } from '../../api/admin'
import type { SecurityPolicy, PermissionRowFilter } from '../../api/admin'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'
import { useAuth } from '../auth/AuthProvider'
import { ReadOnlyNote } from './ReadOnlyNote'
import {
  datasourceSelectOptions,
  securityRoleOptions,
  semanticModelSelectOptions,
} from './adminSelectOptions'
import { FILTER_OPERATOR_OPTIONS, fieldSelectOptions } from './securityPolicyConstants'

export function RowLevelSecurityPanel({ token }: { token: string }) {
  const t = useT()
  const { hasPermission } = useAuth()
  // Row-level security policies are stored as permissions (admin:roles).
  const canEdit = hasPermission('admin:roles')

  // Selectors
  const [selectedRole, setSelectedRole] = useState('viewer')
  const { datasources, loading: loadingDS } = useDatasources()
  const [selectedDS, setSelectedDS] = useState<string>('')

  // Semantic Models
  const { models, loading: loadingModels } = useSemanticModels(selectedDS || null)
  const [selectedModel, setSelectedModel] = useState<string>('')
  const { model, loading: loadingModelDetail } = useModelDetail(selectedModel || null)

  // Policy & Filters
  const [policy, setPolicy] = useState<SecurityPolicy | null>(null)
  const [filters, setFilters] = useState<PermissionRowFilter[]>([])
  const [loadingPolicy, setLoadingPolicy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  // Auto-select first datasource
  useEffect(() => {
    if (datasources && datasources.length > 0 && !selectedDS) {
      const firstDS = datasources[0]
      if (firstDS) {
        setSelectedDS(firstDS.id)
      }
    }
  }, [datasources, selectedDS])

  // Auto-select first model
  useEffect(() => {
    if (models && models.length > 0) {
      const firstModel = models[0]
      if (firstModel) {
        setSelectedModel(firstModel.id)
      }
    } else {
      setSelectedModel('')
    }
  }, [models])

  // Fetch policy when role or datasource changes
  useEffect(() => {
    if (!selectedRole || !selectedDS) {
      setPolicy(null)
      setFilters([])
      return
    }

    let cancelled = false
    async function loadPolicy() {
      setLoadingPolicy(true)
      setError(null)
      setSaveSuccess(false)
      try {
        const policyData = await getSecurityPolicyByKeys(token, `role:${selectedRole}`, selectedDS)
        if (cancelled) return
        setPolicy(policyData)
        setFilters(policyData.row_filters || [])
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err))
        }
      } finally {
        if (!cancelled) {
          setLoadingPolicy(false)
        }
      }
    }

    loadPolicy()
    return () => {
      cancelled = true
    }
  }, [token, selectedRole, selectedDS])

  const fields = useMemo(() => {
    const names: string[] = []
    if (model?.dimensions) {
      model.dimensions.forEach((d) => names.push(d.name))
    }
    if (model?.metrics) {
      model.metrics.forEach((m) => names.push(m.name))
    }
    return names
  }, [model])

  const dsOptions = useMemo(
    () => datasourceSelectOptions(datasources ?? [], loadingDS),
    [datasources, loadingDS],
  )
  const modelOptions = useMemo(
    () => semanticModelSelectOptions(models ?? [], loadingModels),
    [models, loadingModels],
  )
  const fieldOptions = useMemo(() => fieldSelectOptions(fields), [fields])

  const handleAddFilter = () => {
    if (fields.length === 0) return
    const firstField = fields[0] || ''
    setFilters([
      ...filters,
      { field: firstField, operator: 'eq', value: '' },
    ])
    setSaveSuccess(false)
  }

  const handleRemoveFilter = (index: number) => {
    setFilters(filters.filter((_, i) => i !== index))
    setSaveSuccess(false)
  }

  const handleFilterChange = (index: number, key: keyof PermissionRowFilter, val: any) => {
    const updated = [...filters]
    const item = updated[index]
    if (!item) return

    if (key === 'operator') {
      const op = val as string
      if (op === 'is_null' || op === 'is_not_null') {
        updated[index] = { field: item.field, operator: op, value: null }
      } else if (op === 'in' || op === 'not_in') {
        updated[index] = { field: item.field, operator: op, value: [] }
      } else if (op === 'between') {
        updated[index] = { field: item.field, operator: op, value: ['', ''] }
      } else {
        updated[index] = { field: item.field, operator: op, value: '' }
      }
    } else if (key === 'value') {
      const op = item.operator
      if (op === 'in' || op === 'not_in') {
        try {
          const parsed = JSON.parse(val)
          updated[index] = { field: item.field, operator: op, value: Array.isArray(parsed) ? parsed : [val] }
        } catch {
          updated[index] = { field: item.field, operator: op, value: String(val).split(',').map(s => s.trim()) }
        }
      } else {
        updated[index] = { field: item.field, operator: op, value: val }
      }
    } else if (key === 'field') {
      updated[index] = { field: val as string, operator: item.operator, value: item.value }
    }
    setFilters(updated)
    setSaveSuccess(false)
  }

  const handleSave = async () => {
    if (!selectedDS) return
    setError(null)
    setSaveSuccess(false)

    // Construct request policy
    const policyToSave: SecurityPolicy = {
      id: policy?.id,
      user_id: `role:${selectedRole}`,
      datasource_id: selectedDS,
      allowed_models: policy?.allowed_models || [],
      denied_fields: policy?.denied_fields || [],
      row_filters: filters,
    }

    try {
      setLoadingPolicy(true)
      const res = await upsertSecurityPolicy(token, policyToSave)
      setPolicy(res)
      setFilters(res.row_filters || [])
      setSaveSuccess(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoadingPolicy(false)
    }
  }

  const isSavingDisabled = fields.length === 0 || loadingPolicy

  return (
    <div style={containerStyle}>
      <h2 style={headerStyle}>{t('admin.tabs.row_level_security')}</h2>

      {!canEdit && <ReadOnlyNote />}

      <div style={gridSelectStyle}>
        <div style={labelStyle} className="admin-form-label">
          <span style={labelTextStyle}>Role</span>
          <Select value={selectedRole} options={securityRoleOptions()} onChange={setSelectedRole} />
        </div>

        <div style={labelStyle} className="admin-form-label">
          <span style={labelTextStyle}>Datasource</span>
          <Select
            value={selectedDS}
            options={dsOptions}
            onChange={setSelectedDS}
            disabled={loadingDS || dsOptions.every((o) => o.disabled)}
          />
        </div>

        <div style={labelStyle} className="admin-form-label">
          <span style={labelTextStyle}>Semantic Model</span>
          <Select
            value={selectedModel}
            options={modelOptions}
            onChange={setSelectedModel}
            disabled={!selectedDS || loadingModels}
          />
        </div>
      </div>

      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}
      {saveSuccess && <div style={successStyle}>Access policy saved successfully!</div>}

      <div style={contentGridStyle}>
        <div style={leftPanelStyle}>
          <LoadingOverlay loading={loadingPolicy || loadingModelDetail}>
            <div style={innerPanelStyle}>
              <div style={builderHeaderStyle}>
                <h3 style={sectionTitleStyle}>Filter Rules</h3>
                <button
                  onClick={handleAddFilter}
                  disabled={fields.length === 0 || !canEdit}
                  style={fields.length === 0 || !canEdit ? btnSecondaryDisabled : btnAddStyle}
                >
                  + Add Filter Row
                </button>
              </div>

              {fields.length === 0 ? (
                <div style={noFieldsStyle}>
                  Select a semantic model with configured fields to build filters.
                </div>
              ) : filters.length === 0 ? (
                <div style={noFiltersStyle}>
                  No row filters defined for this role and datasource yet. Click "Add Filter Row" to restrict access.
                </div>
              ) : (
                <div style={filtersListStyle}>
                  {filters.map((f, i) => {
                    const isValDisabled = f.operator === 'is_null' || f.operator === 'is_not_null'
                    let valDisplay = ''
                    if (Array.isArray(f.value)) {
                      valDisplay = f.value.join(', ')
                    } else if (f.value !== null && f.value !== undefined) {
                      valDisplay = String(f.value)
                    }

                    return (
                      <div key={i} style={filterRowStyle} className="admin-filter-row">
                        <Select
                          size="sm"
                          value={f.field}
                          options={fieldOptions}
                          onChange={(v) => handleFilterChange(i, 'field', v)}
                          disabled={!canEdit}
                        />

                        <Select
                          size="sm"
                          value={f.operator || 'eq'}
                          options={FILTER_OPERATOR_OPTIONS}
                          onChange={(v) => handleFilterChange(i, 'operator', v)}
                          disabled={!canEdit}
                        />

                        <input
                          type="text"
                          value={isValDisabled ? '' : valDisplay}
                          disabled={isValDisabled || !canEdit}
                          onChange={(e) => handleFilterChange(i, 'value', e.target.value)}
                          placeholder={f.operator === 'in' || f.operator === 'not_in' ? 'val1, val2, val3' : 'Value'}
                          style={isValDisabled ? inputStyleDisabled : filterInputStyle}
                        />

                        <button onClick={() => handleRemoveFilter(i)} style={btnDeleteRowStyle} disabled={!canEdit}>
                          ✕
                        </button>
                      </div>
                    )
                  })}
                </div>
              )}

              <div style={{ marginTop: 24 }}>
                <button
                  onClick={handleSave}
                  disabled={isSavingDisabled || !canEdit}
                  style={isSavingDisabled || !canEdit ? btnPrimaryDisabled : btnPrimary}
                >
                  Save Access Rules
                </button>
              </div>
            </div>
          </LoadingOverlay>
        </div>

        <div style={rightPanelStyle}>
          <h3 style={sectionTitleStyle}>JSON Policy Output</h3>
          <pre style={jsonPreviewStyle}>
            {JSON.stringify(filters, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  )
}

const containerStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 20,
}

const headerStyle: React.CSSProperties = {
  margin: 0,
  fontSize: '20px',
  fontWeight: 600,
  color: 'var(--text-primary, #f4f4f5)',
}

const gridSelectStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
  gap: 16,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.04))',
  padding: 16,
  borderRadius: 8,
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const labelStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 6,
}

const labelTextStyle: React.CSSProperties = {
  fontSize: '12px',
  color: 'var(--text-secondary, #a1a1aa)',
  fontWeight: 500,
  textTransform: 'uppercase',
  letterSpacing: '0.5px',
}

const contentGridStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: '3fr 2fr',
  gap: 24,
  alignItems: 'start',
}

const leftPanelStyle: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}

const innerPanelStyle: React.CSSProperties = {
  padding: 24,
}

const rightPanelStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 12,
}

const builderHeaderStyle: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: 20,
}

const sectionTitleStyle: React.CSSProperties = {
  margin: 0,
  fontSize: 16,
  fontWeight: 600,
  color: 'var(--text-primary, #f4f4f5)',
}

const btnAddStyle: React.CSSProperties = {
  padding: '6px 12px',
  background: 'rgba(99, 102, 241, 0.1)',
  color: 'var(--accent, #6366f1)',
  border: '1px solid var(--accent, #6366f1)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 12,
  fontWeight: 600,
  transition: 'all 150ms ease',
}

const btnSecondaryDisabled: React.CSSProperties = {
  padding: '6px 12px',
  background: 'rgba(255, 255, 255, 0.02)',
  color: 'var(--text-secondary, #a1a1aa)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  cursor: 'not-allowed',
  fontSize: 12,
  opacity: 0.5,
}

const noFieldsStyle: React.CSSProperties = {
  padding: '40px 20px',
  textAlign: 'center',
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 14,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.02))',
  borderRadius: 6,
  border: '1px dashed var(--border, rgba(255, 255, 255, 0.06))',
}

const noFiltersStyle: React.CSSProperties = {
  padding: '40px 20px',
  textAlign: 'center',
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 14,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.02))',
  borderRadius: 6,
  border: '1px dashed var(--border, rgba(255, 255, 255, 0.06))',
}

const filtersListStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 12,
  maxHeight: '400px',
  overflowY: 'auto',
  paddingRight: 4,
}

const filterRowStyle: React.CSSProperties = {
  display: 'flex',
  gap: 12,
  alignItems: 'center',
}

const filterInputStyle: React.CSSProperties = {
  flex: '2',
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  fontSize: 13,
  background: 'var(--input-bg, #18181b)',
  color: 'var(--text-primary, #f4f4f5)',
}

const inputStyleDisabled: React.CSSProperties = {
  flex: '2',
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  fontSize: 13,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.02))',
  color: 'var(--text-secondary, #a1a1aa)',
  cursor: 'not-allowed',
  opacity: 0.5,
}

const btnDeleteRowStyle: React.CSSProperties = {
  background: 'transparent',
  border: 0,
  color: 'var(--error, #ef4444)',
  fontSize: 16,
  cursor: 'pointer',
  padding: '4px 8px',
}

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--accent, #6366f1)',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 600,
  boxShadow: 'var(--shadow-sm, 0 1px 2px rgba(0,0,0,0.05))',
}

const btnPrimaryDisabled: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--border, rgba(255, 255, 255, 0.06))',
  color: 'var(--text-secondary, #a1a1aa)',
  border: 0,
  borderRadius: 6,
  cursor: 'not-allowed',
  fontSize: 13,
  fontWeight: 600,
  opacity: 0.5,
}

const jsonPreviewStyle: React.CSSProperties = {
  margin: 0,
  padding: 16,
  background: '#09090b',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  fontFamily: 'var(--font-mono, monospace)',
  fontSize: 12,
  color: '#34d399',
  overflowX: 'auto',
  maxHeight: '460px',
}

const errStyle: React.CSSProperties = {
  color: 'var(--error, #ef4444)',
  padding: '10px 16px',
  background: 'rgba(239, 68, 68, 0.1)',
  borderRadius: 6,
  border: '1px solid rgba(239, 68, 68, 0.2)',
  fontSize: 13,
  fontWeight: 500,
}

const successStyle: React.CSSProperties = {
  color: 'var(--success, #10b981)',
  padding: '10px 16px',
  background: 'rgba(16, 185, 129, 0.1)',
  borderRadius: 6,
  border: '1px solid rgba(16, 185, 129, 0.2)',
  fontSize: 13,
  fontWeight: 500,
}
