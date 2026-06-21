import { useCallback, useEffect, useMemo, useState } from 'react'

import type { PIIAccessLevel, PIIColumn, PIIColumnAccess, SecurityPolicy } from '../../api/admin'
import { getSecurityPolicyByKeys, listPIIColumns, upsertSecurityPolicy } from '../../api/admin'
import { listSemanticModelFields } from '../../api/semantic'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useDatasources } from '../../hooks/useDatasources'
import { useFetch } from '../../hooks/useFetch'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import type { SemanticModelFieldRow } from '../../types/semantic'
import { pickValidIdOrFirst } from '../../utils/effectiveSelection'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import { adminFormLabelClass } from './adminClasses'
import {
  datasourceSelectOptions,
  securityRoleOptions,
  semanticModelSelectOptions,
} from './adminSelectOptions'
import { ReadOnlyNote } from './ReadOnlyNote'

const DEFAULT_FIELD_PAGE_SIZE = 15
const EMPTY_DENIED_FIELDS: string[] = []
const EMPTY_PII_POLICY: Record<string, PIIColumnAccess> = {}
const EMPTY_PII_COLUMNS: PIIColumn[] = []

// eslint-disable-next-line complexity
export function FieldPermissionPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const { hasPermission } = useAuth()
  // Field-level security policies are stored as permissions (admin:roles).
  const canEdit = hasPermission('admin:roles')

  const [selectedRole, setSelectedRole] = useState('viewer')
  const { datasources, loading: loadingDS } = useDatasources()
  const [selectedDSInput, setSelectedDSInput] = useState<string>('')
  const selectedDS = useMemo(
    () => pickValidIdOrFirst(selectedDSInput, datasources),
    [selectedDSInput, datasources],
  )

  const { models, loading: loadingModels } = useSemanticModels(selectedDS || null)
  const [selectedModelInput, setSelectedModelInput] = useState<string>('')
  const selectedModel = useMemo(
    () => pickValidIdOrFirst(selectedModelInput, models),
    [selectedModelInput, models],
  )

  const [modelName, setModelName] = useState<string | null>(null)
  const [fieldRows, setFieldRows] = useState<SemanticModelFieldRow[]>([])
  const [fieldTotal, setFieldTotal] = useState(0)
  const [fieldPage, setFieldPage] = useState(1)
  const [fieldPageSize] = useState(DEFAULT_FIELD_PAGE_SIZE)
  const [fieldsError, setFieldsError] = useState<string | null>(null)

  const setSelectedDS = useCallback((id: string) => {
    setSelectedDSInput(id)
    setSelectedModelInput('')
    setFieldPage(1)
  }, [])

  const setSelectedModel = useCallback((id: string) => {
    setSelectedModelInput(id)
    setFieldPage(1)
  }, [])

  const policyScopeKey = `${selectedRole}:${selectedDS}`
  const [policyState, setPolicyState] = useState<{
    key: string
    policy: SecurityPolicy | null
    deniedFields: string[]
    piiPolicy: Record<string, PIIColumnAccess>
    piiColumns: PIIColumn[]
  }>({
    key: '',
    policy: null,
    deniedFields: [],
    piiPolicy: {},
    piiColumns: [],
  })
  const policy = policyState.key === policyScopeKey ? policyState.policy : null
  const deniedFields =
    policyState.key === policyScopeKey ? policyState.deniedFields : EMPTY_DENIED_FIELDS
  const piiPolicy = policyState.key === policyScopeKey ? policyState.piiPolicy : EMPTY_PII_POLICY
  const piiColumns = policyState.key === policyScopeKey ? policyState.piiColumns : EMPTY_PII_COLUMNS

  const updatePolicyFields = useCallback(
    (
      patch: Partial<{
        policy: SecurityPolicy | null
        deniedFields: string[]
        piiPolicy: Record<string, PIIColumnAccess>
        piiColumns: PIIColumn[]
      }>,
    ) => {
      setPolicyState((prev) =>
        prev.key === policyScopeKey
          ? { ...prev, ...patch }
          : { ...prev, key: policyScopeKey, ...patch },
      )
    },
    [policyScopeKey],
  )

  const fetchPolicy = useCallback(async () => {
    const [policyData, piiCols] = await Promise.all([
      getSecurityPolicyByKeys(token, `role:${selectedRole}`, selectedDS),
      listPIIColumns(token, selectedDS).catch(() => [] as PIIColumn[]),
    ])
    return { policy: policyData, piiColumns: piiCols }
  }, [token, selectedRole, selectedDS])

  const {
    data: policyLoadData,
    loading: loadingPolicy,
    error: loadError,
  } = useFetch(fetchPolicy, [selectedRole, selectedDS, token], {
    enabled: Boolean(selectedRole && selectedDS),
  })

  const { data: fieldsData, loading: loadingFields } = useFetch(
    () =>
      selectedModel
        ? listSemanticModelFields(selectedModel, fieldPage, fieldPageSize)
        : Promise.resolve({ data: null, error: null }),
    [selectedModel, fieldPage, fieldPageSize],
    { enabled: Boolean(selectedModel) },
  )

  const {
    loading: saving,
    error: mutationError,
    setError: setMutationError,
    run: runSave,
  } = useAsyncState({ useSaving: true })

  const error =
    (loadError ? t('admin.field_permissions.load_failed') : null) ?? fieldsError ?? mutationError

  useEffect(() => {
    if (policyLoadData) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setPolicyState({
        key: policyScopeKey,
        policy: policyLoadData.policy,
        deniedFields: policyLoadData.policy.denied_fields,
        piiPolicy: policyLoadData.policy.pii_policy ?? {},
        piiColumns: policyLoadData.piiColumns,
      })
    } else {
      setPolicyState({
        key: '',
        policy: null,
        deniedFields: [],
        piiPolicy: {},
        piiColumns: [],
      })
    }
  }, [policyLoadData, policyScopeKey])

  useEffect(() => {
    if (fieldsData) {
      const { data, error: fieldsErr } = fieldsData
      if (fieldsErr) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setFieldsError(t('admin.field_permissions.load_failed'))
        setModelName(null)
        setFieldRows([])
        setFieldTotal(0)
      } else if (data) {
        setFieldsError(null)
        setModelName(data.model_name)
        setFieldRows(data.items)
        setFieldTotal(data.total)
      }
    } else {
      setFieldsError(null)
      setModelName(null)
      setFieldRows([])
      setFieldTotal(0)
    }
  }, [fieldsData, t])

  const handleToggleField = (fieldName: string) => {
    if (!modelName) {
      return
    }
    const qualified = `${modelName}.${fieldName}`

    const isDenied = deniedFields.includes(qualified) || deniedFields.includes(fieldName)

    let updated: string[]
    if (isDenied) {
      updated = deniedFields.filter((f) => f !== qualified && f !== fieldName)
    } else {
      updated = [...deniedFields, qualified]
    }

    updatePolicyFields({ deniedFields: updated })
  }

  const handleSave = async () => {
    if (!selectedDS) {
      return
    }
    setMutationError(null)

    const policyToSave: SecurityPolicy = {
      id: policy?.id,
      user_id: `role:${selectedRole}`,
      datasource_id: selectedDS,
      allowed_models: policy?.allowed_models ?? [],
      denied_fields: deniedFields,
      row_filters: policy?.row_filters ?? [],
      pii_policy: piiPolicy,
    }

    await runSave(async () => {
      const res = await upsertSecurityPolicy(token, policyToSave)
      updatePolicyFields({
        policy: res,
        deniedFields: res.denied_fields,
        piiPolicy: res.pii_policy ?? {},
      })
      toast.success(t('admin.field_permissions.saved'))
    })
  }

  // Role defaults mirror the backend pii.DefaultPIIPolicy: admin raw,
  // analyst masked, viewer hidden for sensitive types / masked otherwise.
  const roleDefaultAccess = (piiType: string): PIIAccessLevel => {
    if (selectedRole === 'admin') {
      return 'raw'
    }
    if (selectedRole === 'analyst') {
      return 'masked'
    }
    return ['tc_kimlik_no', 'credit_card_like', 'iban'].includes(piiType) ? 'hidden' : 'masked'
  }

  const piiKey = (col: PIIColumn) => `${col.schema}.${col.table}.${col.column}`

  const piiAccessFor = (col: PIIColumn): PIIAccessLevel | '' =>
    piiPolicy[piiKey(col)]?.access ?? piiPolicy[`${col.table}.${col.column}`]?.access ?? ''

  const handlePIIAccessChange = (col: PIIColumn, access: string) => {
    const key = piiKey(col)
    const next = { ...piiPolicy }
    delete next[`${col.table}.${col.column}`]
    if (access === '') {
      delete next[key]
    } else {
      next[key] = { access: access as PIIAccessLevel }
    }
    updatePolicyFields({ piiPolicy: next })
  }

  const handleBulkApplyDefaults = () => {
    const next: Record<string, PIIColumnAccess> = {}
    for (const col of piiColumns) {
      next[piiKey(col)] = { access: roleDefaultAccess(col.pii_type) }
    }
    updatePolicyFields({ piiPolicy: next })
  }

  // PII badge support: map of "table.column" refs to PII type for the badge
  // shown next to dimension fields backed by PII columns.
  const piiTypeByRef = useMemo(() => {
    const map = new Map<string, string>()
    for (const col of piiColumns) {
      map.set(`${col.table}.${col.column}`, col.pii_type)
      map.set(`${col.schema}.${col.table}.${col.column}`, col.pii_type)
    }
    return map
  }, [piiColumns])

  const isFieldDenied = (fieldName: string) => {
    if (!modelName) {
      return false
    }
    const qualified = `${modelName}.${fieldName}`
    return deniedFields.includes(qualified) || deniedFields.includes(fieldName)
  }

  const hasFields = fieldTotal > 0
  const fieldTotalPages = Math.max(1, Math.ceil(fieldTotal / fieldPageSize))
  const isSavingDisabled = !selectedModel || !modelName || loadingPolicy || saving

  const dsOptions = useMemo(
    () => datasourceSelectOptions(datasources, loadingDS),
    [datasources, loadingDS],
  )
  const modelOptions = useMemo(
    () => semanticModelSelectOptions(models, loadingModels),
    [models, loadingModels],
  )

  return (
    <div style={containerStyle}>
      <h2 style={headerStyle}>{t('admin.tabs.field_permissions')}</h2>

      {!canEdit && <ReadOnlyNote />}

      <div style={gridSelectStyle}>
        <div style={labelStyle} className={adminFormLabelClass}>
          <span style={labelTextStyle}>{t('admin.field_permissions.role')}</span>
          <Select value={selectedRole} options={securityRoleOptions()} onChange={setSelectedRole} />
        </div>

        <div style={labelStyle} className={adminFormLabelClass}>
          <span style={labelTextStyle}>{t('admin.field_permissions.datasource')}</span>
          <Select
            value={selectedDS}
            options={dsOptions}
            onChange={setSelectedDS}
            disabled={loadingDS || dsOptions.every((o) => o.disabled)}
          />
        </div>

        <div style={labelStyle} className={adminFormLabelClass}>
          <span style={labelTextStyle}>{t('admin.field_permissions.semantic_model')}</span>
          <Select
            value={selectedModel}
            options={modelOptions}
            onChange={setSelectedModel}
            disabled={!selectedDS || loadingModels}
          />
        </div>
      </div>

      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}

      <div style={contentLayout}>
        <LoadingOverlay loading={loadingPolicy || loadingFields || saving}>
          <div style={innerPanelStyle}>
            <div style={panelHeaderStyle}>
              <h3 style={sectionTitleStyle}>{t('admin.field_permissions.access_matrix')}</h3>
              {modelName && <span style={badgeStyle}>{modelName}</span>}
            </div>

            {!selectedModel ? (
              <div style={noModelStyle}>{t('admin.field_permissions.select_model')}</div>
            ) : !hasFields && !loadingFields ? (
              <div style={noFieldsStyle}>{t('admin.field_permissions.no_fields')}</div>
            ) : (
              <div style={fieldsTableContainer}>
                <table style={tableStyle}>
                  <thead>
                    <tr style={theadRow}>
                      <th style={thStyle}>{t('admin.field_permissions.col_field')}</th>
                      <th style={thStyle}>{t('admin.field_permissions.col_type')}</th>
                      <th style={thStyle}>{t('admin.field_permissions.col_expression')}</th>
                      <th style={{ ...thStyle, textAlign: 'center', width: 120 }}>
                        {t('admin.field_permissions.col_denied')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {fieldRows.map((row) => {
                      const denied = isFieldDenied(row.name)
                      return (
                        <tr key={`${row.kind}-${row.id}`} style={trRow}>
                          <td style={tdStyle}>
                            <div style={fieldNameContainer}>
                              <strong style={nameStyle}>
                                {row.name}
                                {piiTypeByRef.has(row.ref) && (
                                  <span
                                    style={piiBadgeStyle}
                                    title={`${t('admin.pii.badge')}: ${piiTypeByRef.get(row.ref)}`}
                                  >
                                    {t('admin.pii.badge')}
                                  </span>
                                )}
                              </strong>
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

            {piiColumns.length > 0 && (
              <div style={{ marginTop: 32 }}>
                <div style={panelHeaderStyle}>
                  <h3 style={sectionTitleStyle}>{t('admin.pii.policy_title')}</h3>
                </div>
                <p style={piiDescStyle}>{t('admin.pii.policy_description')}</p>
                <div style={fieldsTableContainer}>
                  <table style={tableStyle}>
                    <thead>
                      <tr style={theadRow}>
                        <th style={thStyle}>{t('admin.pii.col_column')}</th>
                        <th style={thStyle}>{t('admin.pii.col_type')}</th>
                        <th style={{ ...thStyle, width: 200 }}>{t('admin.pii.col_access')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {piiColumns.map((col) => (
                        <tr key={col.column_id} style={trRow}>
                          <td style={tdStyle}>
                            <code style={codeStyle}>
                              {col.schema}.{col.table}.{col.column}
                            </code>
                          </td>
                          <td style={tdStyle}>
                            <span style={dimTypeBadge}>{col.pii_type}</span>
                          </td>
                          <td style={tdStyle}>
                            <select
                              value={piiAccessFor(col)}
                              onChange={(e) => handlePIIAccessChange(col, e.target.value)}
                              disabled={!canEdit}
                              style={piiSelectStyle}
                              aria-label={t('admin.field_permissions.access', {
                                column: col.column,
                              })}
                            >
                              <option value="">
                                {t('admin.pii.policy_default')} ({roleDefaultAccess(col.pii_type)})
                              </option>
                              <option value="raw">{t('admin.pii.access_raw')}</option>
                              <option value="masked">{t('admin.pii.access_masked')}</option>
                              <option value="hidden">{t('admin.pii.access_hidden')}</option>
                            </select>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div style={{ marginTop: 12 }}>
                  <button
                    onClick={handleBulkApplyDefaults}
                    disabled={!canEdit}
                    style={!canEdit ? btnSecondaryDisabled : btnSecondary}
                  >
                    {t('admin.pii.bulk_apply', { role: selectedRole })}
                  </button>
                </div>
              </div>
            )}

            <div style={{ marginTop: 24 }}>
              <button
                onClick={() => {
                  void handleSave()
                }}
                disabled={isSavingDisabled || !canEdit}
                style={isSavingDisabled || !canEdit ? btnPrimaryDisabled : btnPrimary}
              >
                {t('admin.field_permissions.save')}
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
  color: 'var(--accent)',
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
  background: 'var(--accent)',
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

const piiBadgeStyle: React.CSSProperties = {
  marginLeft: 8,
  padding: '1px 6px',
  background: 'rgba(239, 68, 68, 0.12)',
  color: '#ef4444',
  borderRadius: 999,
  fontSize: 10,
  fontWeight: 700,
  letterSpacing: '0.5px',
  verticalAlign: 'middle',
}

const piiDescStyle: React.CSSProperties = {
  margin: '0 0 12px',
  fontSize: 13,
  color: 'var(--text-secondary, #a1a1aa)',
}

const piiSelectStyle: React.CSSProperties = {
  padding: '6px 10px',
  borderRadius: 6,
  border: '1px solid var(--border, rgba(255, 255, 255, 0.12))',
  background: 'var(--bg-card, transparent)',
  color: 'var(--text-primary, #f4f4f5)',
  fontSize: 13,
  minWidth: 180,
}

const btnSecondary: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: 'var(--accent)',
  border: '1px solid var(--accent)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 12,
  fontWeight: 600,
}

const btnSecondaryDisabled: React.CSSProperties = {
  ...btnSecondary,
  color: 'var(--text-secondary, #a1a1aa)',
  border: '1px solid var(--border, rgba(255,255,255,0.1))',
  cursor: 'not-allowed',
  opacity: 0.5,
}
