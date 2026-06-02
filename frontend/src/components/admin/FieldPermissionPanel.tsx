import { useCallback, useEffect, useMemo, useState } from 'react'
import { useT } from '../../i18n'
import { useDatasources } from '../../hooks/useDatasources'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { request } from '../../hooks/useApi'
import { getSecurityPolicyByKeys, upsertSecurityPolicy } from '../../api/admin'
import type { SecurityPolicy } from '../../api/admin'
import type { SemanticModelFieldRow, SemanticModelFieldsPage } from '../../types/semantic'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import { useAuth } from '../auth/AuthProvider'
import { ReadOnlyNote } from './ReadOnlyNote'
import {
  datasourceSelectOptions,
  securityRoleOptions,
  semanticModelSelectOptions,
} from './adminSelectOptions'

const DEFAULT_FIELD_PAGE_SIZE = 15

export function FieldPermissionPanel({ token }: { token: string }) {
  const t = useT()
  const { hasPermission } = useAuth()
  // Field-level security policies are stored as permissions (admin:roles).
  const canEdit = hasPermission('admin:roles')

  const [selectedRole, setSelectedRole] = useState('viewer')
  const { datasources, loading: loadingDS } = useDatasources()
  const [selectedDS, setSelectedDS] = useState<string>('')

  const { models, loading: loadingModels } = useSemanticModels(selectedDS || null)
  const [selectedModel, setSelectedModel] = useState<string>('')

  const [policy, setPolicy] = useState<SecurityPolicy | null>(null)
  const [deniedFields, setDeniedFields] = useState<string[]>([])
  const [loadingPolicy, setLoadingPolicy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [modelName, setModelName] = useState<string | null>(null)
  const [fieldRows, setFieldRows] = useState<SemanticModelFieldRow[]>([])
  const [fieldTotal, setFieldTotal] = useState(0)
  const [fieldPage, setFieldPage] = useState(1)
  const [fieldPageSize] = useState(DEFAULT_FIELD_PAGE_SIZE)
  const [loadingFields, setLoadingFields] = useState(false)

  useEffect(() => {
    if (datasources && datasources.length > 0 && !selectedDS) {
      const firstDS = datasources[0]
      if (firstDS) {
        setSelectedDS(firstDS.id)
      }
    }
  }, [datasources, selectedDS])

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

  const loadFields = useCallback(async () => {
    if (!selectedModel) {
      setModelName(null)
      setFieldRows([])
      setFieldTotal(0)
      return
    }
    setLoadingFields(true)
    const params = new URLSearchParams({
      page: String(fieldPage),
      page_size: String(fieldPageSize),
    })
    const { data, error: fieldsErr } = await request<SemanticModelFieldsPage>(
      'GET',
      `/api/semantic/models/${encodeURIComponent(selectedModel)}/fields?${params}`,
    )
    if (fieldsErr) {
      setError(fieldsErr)
      setModelName(null)
      setFieldRows([])
      setFieldTotal(0)
    } else if (data) {
      setModelName(data.model_name)
      setFieldRows(data.items)
      setFieldTotal(data.total)
    } else {
      setModelName(null)
      setFieldRows([])
      setFieldTotal(0)
    }
    setLoadingFields(false)
  }, [selectedModel, fieldPage, fieldPageSize])

  useEffect(() => {
    void loadFields()
  }, [loadFields])

  useEffect(() => {
    setFieldPage(1)
  }, [selectedModel, selectedRole, selectedDS])

  const handleToggleField = (fieldName: string) => {
    if (!modelName) return
    const qualified = `${modelName}.${fieldName}`

    const isDenied = deniedFields.includes(qualified) || deniedFields.includes(fieldName)

    let updated: string[]
    if (isDenied) {
      updated = deniedFields.filter((f) => f !== qualified && f !== fieldName)
    } else {
      updated = [...deniedFields, qualified]
    }

    setDeniedFields(updated)
    setSaveSuccess(false)
  }

  const handleSave = async () => {
    if (!selectedDS) return
    setError(null)
    setSaveSuccess(false)

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
    if (!modelName) return false
    const qualified = `${modelName}.${fieldName}`
    return deniedFields.includes(qualified) || deniedFields.includes(fieldName)
  }

  const hasFields = fieldTotal > 0
  const fieldTotalPages = Math.max(1, Math.ceil(fieldTotal / fieldPageSize))
  const isSavingDisabled = !selectedModel || !modelName || loadingPolicy

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
      {saveSuccess && <div style={successStyle}>Field permissions saved successfully!</div>}

      <div style={contentLayout}>
        <LoadingOverlay loading={loadingPolicy || loadingFields}>
          <div style={innerPanelStyle}>
            <div style={panelHeaderStyle}>
              <h3 style={sectionTitleStyle}>Dimension & Metric Access Matrix</h3>
              {modelName && <span style={badgeStyle}>{modelName}</span>}
            </div>

            {!selectedModel ? (
              <div style={noModelStyle}>
                Select a semantic model to configure field access controls.
              </div>
            ) : !hasFields && !loadingFields ? (
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
                    {fieldRows.map((row) => {
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
                              <span style={dimTypeBadge}>dimension ({row.subtype})</span>
                            ) : (
                              <span style={metricTypeBadge}>metric ({row.subtype})</span>
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
                              disabled={!canEdit}
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
                  totalItems={fieldTotal}
                  itemsPerPage={fieldPageSize}
                  alwaysShow
                />
              </div>
            )}

            <div style={{ marginTop: 24 }}>
              <button
                onClick={handleSave}
                disabled={isSavingDisabled || !canEdit}
                style={isSavingDisabled || !canEdit ? btnPrimaryDisabled : btnPrimary}
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
