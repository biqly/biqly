import { useEffect, useMemo, useState } from 'react'

import type { PermissionRowFilter, SecurityPolicy } from '../../api/admin'
import { getSecurityPolicyByKeys, upsertSecurityPolicy } from '../../api/admin'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useDatasources } from '../../hooks/useDatasources'
import { useFetch } from '../../hooks/useFetch'
import { useModelDetail } from '../../hooks/useModelDetail'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'
import { adminBtnPrimaryClass, adminFilterRowClass, adminFormLabelClass } from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
import {
  datasourceSelectOptions,
  securityRoleOptions,
  semanticModelSelectOptions,
} from './adminSelectOptions'
import { fieldSelectOptions, FILTER_OPERATOR_OPTIONS } from './securityPolicyConstants'

// eslint-disable-next-line complexity
export function RowLevelSecurityPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const { hasPermission } = useAuth()
  // Row-level security policies are stored as permissions (admin:roles).
  const canEdit = hasPermission('admin:roles')

  // Selectors
  const [selectedRole, setSelectedRole] = useState('viewer')
  const { datasources, loading: loadingDS } = useDatasources()
  const [selectedDS, setSelectedDS] = useState<string>('')
  const effectiveSelectedDS = selectedDS.length > 0 ? selectedDS : (datasources[0]?.id ?? '')

  // Semantic Models
  const { models, loading: loadingModels } = useSemanticModels(effectiveSelectedDS || null)
  const [selectedModel, setSelectedModel] = useState<string>('')
  const effectiveSelectedModel = selectedModel.length > 0 ? selectedModel : (models[0]?.id ?? '')
  const { model, loading: loadingModelDetail } = useModelDetail(effectiveSelectedModel || null)

  // Policy & Filters
  const [filters, setFilters] = useState<PermissionRowFilter[]>([])

  const {
    data: policy,
    loading: loadingPolicy,
    error: loadError,
    setData: setPolicy,
  } = useFetch(
    () => getSecurityPolicyByKeys(token, `role:${selectedRole}`, effectiveSelectedDS),
    [token, selectedRole, effectiveSelectedDS],
    { enabled: Boolean(selectedRole && effectiveSelectedDS) },
  )

  const {
    loading: saving,
    error: saveError,
    setError: setSaveError,
    run: runSave,
  } = useAsyncState({ useSaving: true })
  const error = (loadError ? t('admin.rls.load_failed') : null) ?? saveError

  // Fetch policy when role or datasource changes
  useEffect(() => {
    if (policy) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setFilters(policy.row_filters)
    } else {
      setFilters([])
    }
  }, [policy])

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
    () => datasourceSelectOptions(datasources, loadingDS),
    [datasources, loadingDS],
  )
  const modelOptions = useMemo(
    () => semanticModelSelectOptions(models, loadingModels),
    [models, loadingModels],
  )
  const fieldOptions = useMemo(() => fieldSelectOptions(fields), [fields])

  const handleAddFilter = () => {
    if (fields.length === 0) {
      return
    }
    const firstField = fields[0] ?? ''
    setFilters([...filters, { field: firstField, operator: 'eq', value: '' }])
  }

  const handleRemoveFilter = (index: number) => {
    setFilters(filters.filter((_, i) => i !== index))
  }

  const handleFilterChange = (index: number, key: keyof PermissionRowFilter, val: string) => {
    const updated = [...filters]
    const item = updated[index]
    if (!item) {
      return
    }

    if (key === 'operator') {
      const op = val
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
          const parsed: unknown = JSON.parse(val)
          updated[index] = {
            field: item.field,
            operator: op,
            value: Array.isArray(parsed) ? parsed.map(String) : [val],
          }
        } catch {
          updated[index] = {
            field: item.field,
            operator: op,
            value: String(val)
              .split(',')
              .map((s) => s.trim()),
          }
        }
      } else {
        updated[index] = { field: item.field, operator: op, value: val }
      }
    } else {
      updated[index] = { field: val, operator: item.operator, value: item.value }
    }
    setFilters(updated)
  }

  const handleSave = async () => {
    if (!effectiveSelectedDS) {
      return
    }
    setSaveError(null)

    // Construct request policy
    const policyToSave: SecurityPolicy = {
      id: policy?.id,
      user_id: `role:${selectedRole}`,
      datasource_id: effectiveSelectedDS,
      allowed_models: policy?.allowed_models ?? [],
      denied_fields: policy?.denied_fields ?? [],
      row_filters: filters,
    }

    await runSave(async () => {
      const res = await upsertSecurityPolicy(token, policyToSave)
      setPolicy(res)
      toast.success(t('admin.rls.saved'))
    })
  }

  const isSavingDisabled = fields.length === 0 || loadingPolicy || saving

  return (
    <AdminPanelShell title={t('admin.tabs.row_level_security')} readOnly={!canEdit} maxWidth="100%">
      <div className="bg-card-raised border-border grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4 rounded-lg border p-4">
        <label className={adminFormLabelClass}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.rls.role')}
          </span>
          <Select value={selectedRole} options={securityRoleOptions()} onChange={setSelectedRole} />
        </label>

        <label className={adminFormLabelClass}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.rls.datasource')}
          </span>
          <Select
            value={effectiveSelectedDS}
            options={dsOptions}
            onChange={(value) => {
              setSelectedDS(value)
              setSelectedModel('')
              setPolicy(null)
              setFilters([])
            }}
            disabled={loadingDS || dsOptions.every((o) => o.disabled)}
          />
        </label>

        <label className={adminFormLabelClass}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.rls.semantic_model')}
          </span>
          <Select
            value={effectiveSelectedModel}
            options={modelOptions}
            onChange={setSelectedModel}
            disabled={!effectiveSelectedDS || loadingModels}
          />
        </label>
      </div>

      {error && <ErrorAlert error={error} />}

      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[3fr_2fr]">
        <div className="bg-card border-border overflow-hidden rounded-lg border shadow-sm">
          <LoadingOverlay loading={loadingPolicy || loadingModelDetail || saving}>
            <div className="p-6">
              <div className="mb-5 flex items-center justify-between">
                <h3 className="text-foreground m-0 text-base font-semibold">
                  {t('admin.rls.filter_rules')}
                </h3>
                <button
                  onClick={handleAddFilter}
                  disabled={fields.length === 0 || !canEdit}
                  className={cn(
                    'border-accent bg-accent/10 text-accent hover:bg-accent/20 cursor-pointer rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50',
                  )}
                >
                  {t('admin.rls.add_filter')}
                </button>
              </div>

              {fields.length === 0 ? (
                <div className="text-foreground-muted bg-card-raised border-border rounded-lg border border-dashed p-[40px_20px] text-center text-sm">
                  {t('admin.rls.no_fields')}
                </div>
              ) : filters.length === 0 ? (
                <div className="text-foreground-muted bg-card-raised border-border rounded-lg border border-dashed p-[40px_20px] text-center text-sm">
                  {t('admin.rls.no_filters')}
                </div>
              ) : (
                <div className="flex max-h-100 flex-col gap-3 overflow-y-auto pr-1">
                  {filters.map((f, i) => {
                    const isValDisabled = f.operator === 'is_null' || f.operator === 'is_not_null'
                    let valDisplay = ''
                    if (Array.isArray(f.value)) {
                      valDisplay = f.value.join(', ')
                    } else if (f.value !== null && f.value !== undefined) {
                      valDisplay = String(f.value)
                    }

                    return (
                      <div key={i} className={cn(adminFilterRowClass, 'flex items-center gap-3')}>
                        <Select
                          size="sm"
                          value={f.field}
                          options={fieldOptions}
                          onChange={(v) => handleFilterChange(i, 'field', v)}
                          disabled={!canEdit}
                        />

                        <Select
                          size="sm"
                          value={f.operator ?? 'eq'}
                          options={FILTER_OPERATOR_OPTIONS}
                          onChange={(v) => handleFilterChange(i, 'operator', v)}
                          disabled={!canEdit}
                        />

                        <input
                          type="text"
                          value={isValDisabled ? '' : valDisplay}
                          disabled={isValDisabled || !canEdit}
                          onChange={(e) => handleFilterChange(i, 'value', e.target.value)}
                          placeholder={
                            f.operator === 'in' || f.operator === 'not_in'
                              ? t('admin.rls.list_placeholder')
                              : t('admin.rls.value_placeholder')
                          }
                          className={cn(
                            'bg-card text-foreground border-border flex-2 rounded-lg border p-2 text-sm focus-visible:outline-none',
                            (isValDisabled || !canEdit) &&
                              'bg-card-raised text-foreground-muted cursor-not-allowed opacity-50',
                          )}
                        />

                        <button
                          onClick={() => handleRemoveFilter(i)}
                          className="text-error cursor-pointer border-0 bg-transparent p-1 px-2 text-lg transition-colors hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={!canEdit}
                        >
                          ✕
                        </button>
                      </div>
                    )
                  })}
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
                  {t('admin.rls.save_rules')}
                </button>
              </div>
            </div>
          </LoadingOverlay>
        </div>

        <div className="flex flex-col gap-3">
          <h3 className="text-foreground m-0 text-base font-semibold">
            {t('admin.rls.json_policy_output')}
          </h3>
          <pre className="border-border m-0 max-h-115 overflow-x-auto rounded-lg border bg-[#09090b] p-4 font-mono text-xs text-emerald-400">
            {JSON.stringify(filters, null, 2)}
          </pre>
        </div>
      </div>
    </AdminPanelShell>
  )
}
