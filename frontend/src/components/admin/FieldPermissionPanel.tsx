import { useEffect, useMemo, useState } from 'react'
import { useT } from '../../i18n'
import { useDatasources } from '../../hooks/useDatasources'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useModelDetail } from '../../hooks/useModelDetail'
import { getSecurityPolicyByKeys, upsertSecurityPolicy } from '../../api/admin'
import type { SecurityPolicy } from '../../api/admin'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  datasourceSelectOptions,
  securityRoleOptions,
  semanticModelSelectOptions,
} from './adminSelectOptions'

export function FieldPermissionPanel({ token }: { token: string }) {
  const t = useT()

  // Selectors
  const [selectedRole, setSelectedRole] = useState('viewer')
  const { datasources, loading: loadingDS } = useDatasources()
  const [selectedDS, setSelectedDS] = useState<string>('')

  // Semantic Models
  const { models, loading: loadingModels } = useSemanticModels(selectedDS || null)
  const [selectedModel, setSelectedModel] = useState<string>('')
  const { model, loading: loadingModelDetail } = useModelDetail(selectedModel || null)

  // Policy & Denied fields
  const [policy, setPolicy] = useState<SecurityPolicy | null>(null)
  const [deniedFields, setDeniedFields] = useState<string[]>([])
  const [loadingPolicy, setLoadingPolicy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [fieldPage, setFieldPage] = useState(1)
  const fieldPageSize = 15

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
      setDeniedFields([])
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
        setDeniedFields(policyData.denied_fields || [])
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

  const handleToggleField = (fieldName: string) => {
    if (!model) return
    const qualified = `${model.name}.${fieldName}`
    
    // Check if either qualified or plain field is in deniedFields
    const isDenied = deniedFields.includes(qualified) || deniedFields.includes(fieldName)
    
    let updated: string[]
    if (isDenied) {
      // Remove both forms
      updated = deniedFields.filter((f) => f !== qualified && f !== fieldName)
    } else {
      // Add qualified form
      updated = [...deniedFields, qualified]
    }
    
    setDeniedFields(updated)
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
      denied_fields: deniedFields,
      row_filters: policy?.row_filters || [],
    }

    try {
      setLoadingPolicy(true)
      const res = await upsertSecurityPolicy(token, policyToSave)
      setPolicy(res)
      setDeniedFields(res.denied_fields || [])
      setSaveSuccess(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoadingPolicy(false)
    }
  }

  const isFieldDenied = (fieldName: string) => {
    if (!model) return false
    const qualified = `${model.name}.${fieldName}`
    return deniedFields.includes(qualified) || deniedFields.includes(fieldName)
  }

  const hasFields = model && ((model.dimensions?.length || 0) > 0 || (model.metrics?.length || 0) > 0)
  const isSavingDisabled = !model || loadingPolicy

  type FieldRow =
    | { kind: 'dimension'; id: string; name: string; label?: string | null; type: string; ref: string }
    | { kind: 'metric'; id: string; name: string; label?: string | null; aggregation: string; ref: string }

  const fieldRows = useMemo((): FieldRow[] => {
    if (!model) return []
    const dims: FieldRow[] = (model.dimensions ?? []).map((d) => ({
      kind: 'dimension',
      id: d.id,
      name: d.name,
      label: d.label,
      type: d.type,
      ref: d.column_ref,
    }))
    const mets: FieldRow[] = (model.metrics ?? []).map((m) => ({
      kind: 'metric',
      id: m.id,
      name: m.name,
      label: m.label,
      aggregation: m.aggregation,
      ref: m.expression,
    }))
    return [...dims, ...mets]
  }, [model])

  const fieldTotalPages = Math.max(1, Math.ceil(fieldRows.length / fieldPageSize))
  const pagedFieldRows = useMemo(() => {
    const start = (fieldPage - 1) * fieldPageSize
    return fieldRows.slice(start, start + fieldPageSize)
  }, [fieldRows, fieldPage, fieldPageSize])

  useEffect(() => {
    setFieldPage(1)
  }, [selectedModel, selectedRole, selectedDS])

  const dsOptions = useMemo(
    () => datasourceSelectOptions(datasources ?? [], loadingDS),
    [datasources, loadingDS],
  )
  const modelOptions = useMemo(
    () => semanticModelSelectOptions(models ?? [], loadingModels),
    [models, loadingModels],
  )

  return (
    <div style={containerStyle}>
      <h2 style={headerStyle}>{t('admin.tabs.field_permissions')}</h2>

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
      {saveSuccess && <div style={successStyle}>Field permissions saved successfully!</div>}

      <div style={contentLayout}>
        <LoadingOverlay loading={loadingPolicy || loadingModelDetail}>
          <div style={innerPanelStyle}>
            <div style={panelHeaderStyle}>
              <h3 style={sectionTitleStyle}>Dimension & Metric Access Matrix</h3>
              {model && <span style={badgeStyle}>{model.name}</span>}
            </div>

            {!model ? (
              <div style={noModelStyle}>
                Select a semantic model to configure field access controls.
              </div>
            ) : !hasFields ? (
              <div style={noFieldsStyle}>
                This semantic model has no dimensions or metrics configured.
              </div>
            ) : (
              <div style={fieldsTableContainer}>
                <table style={tableStyle}>
                  <thead>
                    <tr style={theadRow}>
                      <th style={thStyle}>Field Name</th>
                      <th style={thStyle}>Type</th>
                      <th style={thStyle}>Expression / Column Ref</th>
                      <th style={{ ...thStyle, textAlign: 'center', width: 120 }}>Denied</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pagedFieldRows.map((row) => {
                      const denied = isFieldDenied(row.name)
                      return (
                        <tr key={`${row.kind}-${row.id}`} style={trRow}>
                          <td style={tdStyle}>
                            <div style={fieldNameContainer}>
                              <strong style={nameStyle}>{row.name}</strong>
                              {row.label && <span style={labelSpan}>{row.label}</span>}
                            </div>
                          </td>
                          <td style={tdStyle}>
                            {row.kind === 'dimension' ? (
                              <span style={dimTypeBadge}>dimension ({row.type})</span>
                            ) : (
                              <span style={metricTypeBadge}>metric ({row.aggregation})</span>
                            )}
                          </td>
                          <td style={tdStyle}>
                            <code style={codeStyle}>{row.ref}</code>
                          </td>
                          <td style={{ ...tdStyle, textAlign: 'center' }}>
                            <input
                              type="checkbox"
                              checked={denied}
                              onChange={() => handleToggleField(row.name)}
                              style={checkboxStyle}
                            />
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
                <Pagination
                  currentPage={fieldPage}
                  totalPages={fieldTotalPages}
                  onPageChange={setFieldPage}
                  totalItems={fieldRows.length}
                  itemsPerPage={fieldPageSize}
                  alwaysShow
                />
              </div>
            )}

            <div style={{ marginTop: 24 }}>
              <button
                onClick={handleSave}
                disabled={isSavingDisabled}
                style={isSavingDisabled ? btnPrimaryDisabled : btnPrimary}
              >
                Save Field Permissions
              </button>
            </div>
          </div>
        </LoadingOverlay>
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

const contentLayout: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}

const innerPanelStyle: React.CSSProperties = {
  padding: 24,
}

const panelHeaderStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  marginBottom: 20,
}

const sectionTitleStyle: React.CSSProperties = {
  margin: 0,
  fontSize: 16,
  fontWeight: 600,
  color: 'var(--text-primary, #f4f4f5)',
}

const badgeStyle: React.CSSProperties = {
  padding: '2px 8px',
  background: 'rgba(99, 102, 241, 0.15)',
  color: 'var(--accent, #6366f1)',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
}

const noModelStyle: React.CSSProperties = {
  padding: '60px 20px',
  textAlign: 'center',
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 14,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.02))',
  borderRadius: 6,
  border: '1px dashed var(--border, rgba(255, 255, 255, 0.06))',
}

const noFieldsStyle: React.CSSProperties = {
  padding: '40px 20px',
  textAlign: 'center',
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 14,
}

const fieldsTableContainer: React.CSSProperties = {
  overflowX: 'auto',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
}

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  fontSize: 14,
  textAlign: 'left',
}

const theadRow: React.CSSProperties = {
  background: 'var(--bg-card-raised, #f9fafb)',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--text-primary, #f4f4f5)',
  fontSize: 13,
}

const trRow: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  color: 'var(--text-primary, #f4f4f5)',
}

const fieldNameContainer: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 2,
}

const nameStyle: React.CSSProperties = {
  fontSize: 14,
  color: 'var(--text-primary, #f4f4f5)',
}

const labelSpan: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-secondary, #a1a1aa)',
}

const dimTypeBadge: React.CSSProperties = {
  padding: '2px 6px',
  background: 'rgba(59, 130, 246, 0.1)',
  color: '#3b82f6',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 500,
}

const metricTypeBadge: React.CSSProperties = {
  padding: '2px 6px',
  background: 'rgba(16, 185, 129, 0.1)',
  color: '#10b981',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 500,
}

const codeStyle: React.CSSProperties = {
  fontFamily: 'var(--font-mono, monospace)',
  fontSize: 12,
  color: 'var(--text-secondary, #a1a1aa)',
  background: 'var(--bg-card-raised, rgba(0, 0, 0, 0.2))',
  padding: '2px 6px',
  borderRadius: 4,
}

const checkboxStyle: React.CSSProperties = {
  width: 16,
  height: 16,
  cursor: 'pointer',
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
