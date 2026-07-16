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
import { cn } from '../../lib/cn'
import type { SemanticModelFieldRow } from '../../types/semantic'
import { pickValidIdOrFirst } from '../../utils/effectiveSelection'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import { adminBtnPrimaryClass, adminBtnSecondaryClass, adminFormLabelClass } from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
import {
  datasourceSelectOptions,
  securityRoleOptions,
  semanticModelSelectOptions,
} from './adminSelectOptions'

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
    <AdminPanelShell
      title={t('admin.tabs.field_permissions')}
      description={t('admin.field_permissions.description')}
      readOnly={!canEdit}
      maxWidth="100%"
    >
      <div className="bg-card-raised border-border grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4 rounded-lg border p-4">
        <label className={adminFormLabelClass}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.field_permissions.role')}
          </span>
          <Select value={selectedRole} options={securityRoleOptions()} onChange={setSelectedRole} />
        </label>

        <label className={adminFormLabelClass}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.field_permissions.datasource')}
          </span>
          <Select
            value={selectedDS}
            options={dsOptions}
            onChange={setSelectedDS}
            disabled={loadingDS || dsOptions.every((o) => o.disabled)}
          />
        </label>

        <label className={adminFormLabelClass}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.field_permissions.semantic_model')}
          </span>
          <Select
            value={selectedModel}
            options={modelOptions}
            onChange={setSelectedModel}
            disabled={!selectedDS || loadingModels}
          />
        </label>
      </div>

      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}

      <div className="bg-card border-border overflow-hidden rounded-lg border shadow-sm">
        <LoadingOverlay loading={loadingPolicy || loadingFields || saving}>
          <div className="p-6">
            <div className="mb-5 flex flex-wrap items-center gap-3">
              <h3 className="text-foreground m-0 text-base font-semibold">
                {t('admin.field_permissions.access_matrix')}
              </h3>
              {modelName && (
                <span className="bg-accent/15 text-accent inline-block rounded px-2 py-0.5 text-xs font-semibold">
                  {modelName}
                </span>
              )}
            </div>

            {!selectedModel ? (
              <div className="text-foreground-muted bg-card-raised border-border rounded-lg border border-dashed p-[60px_20px] text-center text-sm">
                {t('admin.field_permissions.select_model')}
              </div>
            ) : !hasFields && !loadingFields ? (
              <div className="text-foreground-muted p-[40px_20px] text-center text-sm">
                {t('admin.field_permissions.no_fields')}
              </div>
            ) : (
              <div className="border-border overflow-x-auto rounded-lg border">
                <table className="w-full border-collapse text-left text-sm">
                  <thead>
                    <tr className="border-border bg-card-raised border-b">
                      <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                        {t('admin.field_permissions.col_field')}
                      </th>
                      <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                        {t('admin.field_permissions.col_type')}
                      </th>
                      <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                        {t('admin.field_permissions.col_expression')}
                      </th>
                      <th className="text-foreground w-30 p-[12px_16px] text-center text-xs font-semibold tracking-wider uppercase">
                        {t('admin.field_permissions.col_denied')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {fieldRows.map((row) => {
                      const denied = isFieldDenied(row.name)
                      return (
                        <tr key={`${row.kind}-${row.id}`} className="border-border border-b">
                          <td className="text-foreground p-[12px_16px]">
                            <div className="flex flex-col gap-0.5">
                              <strong className="text-foreground flex items-center text-sm font-semibold">
                                {row.name}
                                {piiTypeByRef.has(row.ref) && (
                                  <span
                                    className="text-error text-micro ml-2 inline-block rounded-full bg-red-500/12 px-1.5 py-0.5 font-bold tracking-wide uppercase"
                                    title={`${t('admin.pii.badge')}: ${piiTypeByRef.get(row.ref)}`}
                                  >
                                    {t('admin.pii.badge')}
                                  </span>
                                )}
                              </strong>
                              {row.label && (
                                <span className="text-foreground-muted text-xs">{row.label}</span>
                              )}
                            </div>
                          </td>
                          <td className="text-foreground p-[12px_16px]">
                            {row.kind === 'dimension' ? (
                              <span className="text-2xs inline-block rounded bg-blue-500/10 px-1.5 py-0.5 font-medium text-blue-500 uppercase">
                                dimension ({row.subtype})
                              </span>
                            ) : (
                              <span className="text-success text-2xs inline-block rounded bg-emerald-500/10 px-1.5 py-0.5 font-medium uppercase">
                                metric ({row.subtype})
                              </span>
                            )}
                          </td>
                          <td className="text-foreground p-[12px_16px]">
                            <code className="text-foreground-muted bg-card-raised rounded px-1.5 py-0.5 font-mono text-xs">
                              {row.ref}
                            </code>
                          </td>
                          <td className="text-foreground p-[12px_16px] text-center">
                            <input
                              type="checkbox"
                              checked={denied}
                              onChange={() => handleToggleField(row.name)}
                              disabled={!canEdit}
                              className="size-4 cursor-pointer"
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
              <div className="mt-8">
                <div className="mb-5 flex items-center gap-3">
                  <h3 className="text-foreground m-0 text-base font-semibold">
                    {t('admin.pii.policy_title')}
                  </h3>
                </div>
                <p className="text-foreground-muted m-0 mb-3 text-sm">
                  {t('admin.pii.policy_description')}
                </p>
                <div className="border-border overflow-x-auto rounded-lg border">
                  <table className="w-full border-collapse text-left text-sm">
                    <thead>
                      <tr className="border-border bg-card-raised border-b">
                        <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                          {t('admin.pii.col_column')}
                        </th>
                        <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                          {t('admin.pii.col_type')}
                        </th>
                        <th className="text-foreground w-50 p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                          {t('admin.pii.col_access')}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {piiColumns.map((col) => (
                        <tr key={col.column_id} className="border-border border-b">
                          <td className="text-foreground p-[12px_16px]">
                            <code className="text-foreground-muted bg-card-raised rounded px-1.5 py-0.5 font-mono text-xs">
                              {col.schema}.{col.table}.{col.column}
                            </code>
                          </td>
                          <td className="text-foreground p-[12px_16px]">
                            <span className="text-2xs inline-block rounded bg-blue-500/10 px-1.5 py-0.5 font-medium text-blue-500 uppercase">
                              {col.pii_type}
                            </span>
                          </td>
                          <td className="text-foreground p-[12px_16px]">
                            <Select
                              value={piiAccessFor(col)}
                              onChange={(v) => handlePIIAccessChange(col, v)}
                              disabled={!canEdit}
                              size="sm"
                              options={[
                                {
                                  value: '',
                                  label: `${t('admin.pii.policy_default')} (${roleDefaultAccess(col.pii_type)})`,
                                },
                                { value: 'raw', label: t('admin.pii.access_raw') },
                                { value: 'masked', label: t('admin.pii.access_masked') },
                                { value: 'hidden', label: t('admin.pii.access_hidden') },
                              ]}
                              ariaLabel={t('admin.field_permissions.access', {
                                column: col.column,
                              })}
                            />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="mt-3">
                  <button
                    onClick={handleBulkApplyDefaults}
                    disabled={!canEdit}
                    className={cn(
                      adminBtnSecondaryClass,
                      !canEdit && 'cursor-not-allowed opacity-50',
                    )}
                  >
                    {t('admin.pii.bulk_apply', { role: selectedRole })}
                  </button>
                </div>
              </div>
            )}

            <div className="mt-6">
              <button
                onClick={() => {
                  void handleSave()
                }}
                disabled={isSavingDisabled || !canEdit}
                className={cn(
                  adminBtnPrimaryClass,
                  (isSavingDisabled || !canEdit) && 'cursor-not-allowed opacity-50',
                )}
              >
                {t('admin.field_permissions.save')}
              </button>
            </div>
          </div>
        </LoadingOverlay>
      </div>
    </AdminPanelShell>
  )
}
