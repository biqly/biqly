import { useMemo, useRef, useState } from 'react'

import {
  type FunctionBlocklist,
  getFunctionBlocklist,
  updateFunctionBlocklist,
} from '../../api/functionBlocklist'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { formControlClass, formHintClass, formLabelClass } from '../../lib/formClasses'
import type { Datasource } from '../../types/metadata'
import { ErrorAlert } from '../ui/ErrorAlert'
import { Select } from '../ui/Select'

const SQL_FUNCTION_IDENTIFIER = /^[A-Za-z_][A-Za-z0-9_]*$/

function isSimpleSQLFunctionIdentifier(value: string): boolean {
  return SQL_FUNCTION_IDENTIFIER.test(value)
}

interface FunctionBlocklistPanelProps {
  datasources: Datasource[]
}

export function FunctionBlocklistPanel({ datasources }: FunctionBlocklistPanelProps) {
  const t = useT()
  const requestSequence = useRef(0)
  const [datasourceId, setDatasourceId] = useState('')
  const [blocklist, setBlocklist] = useState<FunctionBlocklist>({ defaults: [], custom: [] })
  const [functionName, setFunctionName] = useState('')
  const [inputError, setInputError] = useState<string | null>(null)
  const [requestError, setRequestError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const datasourceOptions = useMemo(
    () => datasources.map((ds) => ({ value: ds.id, label: `${ds.name} · ${ds.type}` })),
    [datasources],
  )

  const loadBlocklist = async (id: string) => {
    const requestId = requestSequence.current + 1
    requestSequence.current = requestId
    setLoading(true)
    setRequestError(null)
    setSaved(false)
    try {
      const nextBlocklist = await getFunctionBlocklist(id)
      if (requestSequence.current === requestId) {
        setBlocklist(nextBlocklist)
      }
    } catch (err) {
      if (requestSequence.current === requestId) {
        setRequestError(err instanceof Error ? err.message : t('common.unknown_error'))
      }
    } finally {
      if (requestSequence.current === requestId) {
        setLoading(false)
      }
    }
  }

  const selectDatasource = (id: string) => {
    setDatasourceId(id)
    setBlocklist({ defaults: [], custom: [] })
    setFunctionName('')
    setInputError(null)
    setRequestError(null)
    setSaved(false)
    if (id) {
      void loadBlocklist(id)
    }
  }

  const addFunction = () => {
    const normalizedName = functionName.trim().toLowerCase()
    if (!isSimpleSQLFunctionIdentifier(normalizedName)) {
      setInputError(t('datasources.function_blocklist.invalid_identifier'))
      return
    }

    const existingNames = new Set(
      [...blocklist.defaults, ...blocklist.custom].map((name) => name.toLowerCase()),
    )
    if (existingNames.has(normalizedName)) {
      setInputError(t('datasources.function_blocklist.duplicate_identifier'))
      return
    }

    setBlocklist((current) => ({ ...current, custom: [...current.custom, normalizedName] }))
    setFunctionName('')
    setInputError(null)
    setSaved(false)
  }

  const removeFunction = (name: string) => {
    setBlocklist((current) => ({
      ...current,
      custom: current.custom.filter((currentName) => currentName !== name),
    }))
    setSaved(false)
  }

  const saveBlocklist = async () => {
    if (!datasourceId || saving) {
      return
    }
    setSaving(true)
    setRequestError(null)
    setSaved(false)
    try {
      setBlocklist(await updateFunctionBlocklist(datasourceId, blocklist.custom))
      setSaved(true)
    } catch (err) {
      setRequestError(err instanceof Error ? err.message : t('common.unknown_error'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section
      className={cn(cardClass({ elevated: true }), 'flex flex-col gap-4')}
      aria-labelledby="function-blocklist-heading"
    >
      <div>
        <h2 id="function-blocklist-heading">{t('datasources.function_blocklist.title')}</h2>
        <p className="text-foreground-muted mt-1 mb-0 text-sm leading-[1.45]">
          {t('datasources.function_blocklist.hint')}
        </p>
      </div>

      <div className="flex max-w-xl flex-col gap-1.5">
        <label className={formLabelClass} htmlFor="function-blocklist-datasource">
          {t('datasources.function_blocklist.datasource_label')}
        </label>
        <Select
          id="function-blocklist-datasource"
          value={datasourceId}
          onChange={selectDatasource}
          options={datasourceOptions}
          searchable
          placeholder={t('datasources.function_blocklist.datasource_placeholder')}
          disabled={datasources.length === 0}
          ariaLabel={t('datasources.function_blocklist.datasource_label')}
        />
      </div>

      {!datasourceId && (
        <p className="text-foreground-muted m-0 text-sm">
          {t('datasources.function_blocklist.select_datasource')}
        </p>
      )}

      {datasourceId && loading && (
        <p className="text-foreground-muted m-0 text-sm" role="status">
          {t('datasources.function_blocklist.loading')}
        </p>
      )}

      {requestError && <ErrorAlert error={requestError} />}

      {datasourceId && !loading && !requestError && (
        <>
          <FunctionList
            title={t('datasources.function_blocklist.default_title')}
            values={blocklist.defaults}
            hint={t('datasources.function_blocklist.defaults_hint')}
            emptyMessage={t('datasources.function_blocklist.defaults_empty')}
          />

          <div>
            <h3 className="m-0 text-sm font-semibold">
              {t('datasources.function_blocklist.custom_title')}
            </h3>
            <p className={formHintClass}>{t('datasources.function_blocklist.custom_hint')}</p>
            {blocklist.custom.length ? (
              <ul
                className="m-0 flex flex-wrap gap-1.5 p-0"
                aria-label={t('datasources.function_blocklist.custom_title')}
              >
                {blocklist.custom.map((name) => (
                  <li
                    className="border-border bg-card-raised flex list-none items-center gap-1 rounded border py-1 pr-1 pl-2 text-xs"
                    key={name}
                  >
                    <code>{name}</code>
                    <button
                      className={buttonClass('ghost', { size: 'sm' })}
                      type="button"
                      onClick={() => removeFunction(name)}
                      aria-label={t('datasources.function_blocklist.remove_aria', { name })}
                    >
                      {t('datasources.function_blocklist.remove')}
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-foreground-muted m-0 text-sm">
                {t('datasources.function_blocklist.custom_empty')}
              </p>
            )}
          </div>

          <form
            className="flex max-w-xl flex-wrap items-end gap-2"
            onSubmit={(event) => {
              event.preventDefault()
              addFunction()
            }}
          >
            <div className="min-w-60 flex-1">
              <label className={formLabelClass} htmlFor="function-blocklist-name">
                {t('datasources.function_blocklist.function_name_label')}
              </label>
              <input
                id="function-blocklist-name"
                className={formControlClass}
                value={functionName}
                onChange={(event) => {
                  setFunctionName(event.target.value)
                  setInputError(null)
                }}
                placeholder={t('datasources.function_blocklist.function_name_placeholder')}
                aria-describedby={inputError ? 'function-blocklist-input-error' : undefined}
              />
            </div>
            <button type="submit" className={buttonClass('secondary')}>
              {t('datasources.function_blocklist.add')}
            </button>
          </form>
          {inputError && (
            <p id="function-blocklist-input-error" className="text-danger m-0 text-sm" role="alert">
              {inputError}
            </p>
          )}

          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              className={buttonClass('primary')}
              disabled={saving}
              onClick={() => void saveBlocklist()}
            >
              {saving
                ? t('datasources.function_blocklist.saving')
                : t('datasources.function_blocklist.save')}
            </button>
            {saved && (
              <span className="text-success text-sm" role="status">
                {t('datasources.function_blocklist.saved')}
              </span>
            )}
          </div>
        </>
      )}
    </section>
  )
}

function FunctionList({
  title,
  values,
  hint,
  emptyMessage,
}: {
  title: string
  values: string[]
  hint: string
  emptyMessage: string
}) {
  return (
    <div>
      <h3 className="m-0 text-sm font-semibold">{title}</h3>
      <p className={formHintClass}>{hint}</p>
      {values.length ? (
        <ul className="m-0 flex flex-wrap gap-1.5 p-0" aria-label={title}>
          {values.map((name) => (
            <li className="bg-surface-hover list-none rounded px-2 py-1 text-xs" key={name}>
              <code>{name}</code>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-foreground-muted m-0 text-sm">{emptyMessage}</p>
      )}
    </div>
  )
}
