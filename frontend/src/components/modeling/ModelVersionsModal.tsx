import { useCallback, useEffect, useState } from 'react'

import { apiFetchText } from '../../api/apiClient'
import { useApi } from '../../hooks/useApi'
import { useConfirm } from '../../hooks/useConfirm'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { downloadTextFile } from '../../utils/downloadFile'
import { formatDateTime } from '../../utils/formatters'
import { type DiffLine, diffLines } from '../../utils/lineDiff'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'

interface SnapshotInfo {
  version: number
  created_by?: string | null
  created_at: string
}

function diffLineClass(type: DiffLine['type']): string {
  if (type === 'add') {
    return 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
  }
  if (type === 'del') {
    return 'bg-red-500/15 text-red-700 dark:text-red-300'
  }
  return 'text-muted-foreground'
}

function diffLinePrefix(type: DiffLine['type']): string {
  if (type === 'add') {
    return '+ '
  }
  if (type === 'del') {
    return '- '
  }
  return '  '
}

export function ModelVersionsModal({
  open,
  modelId,
  modelName,
  onClose,
  onRolledBack,
}: {
  open: boolean
  modelId: string
  modelName: string
  onClose: () => void
  /** Called after a successful rollback so the page reloads the model. */
  onRolledBack: (version: number) => void
}) {
  const t = useT()
  const [locale] = useLocale()
  const languageTag = localeLanguageTag(locale)
  const confirm = useConfirm()
  const { get, postData, loading, error } = useApi()
  const [versions, setVersions] = useState<SnapshotInfo[]>([])
  const [fromVersion, setFromVersion] = useState('')
  const [toVersion, setToVersion] = useState('')
  const [diff, setDiff] = useState<DiffLine[] | null>(null)
  const [busyVersion, setBusyVersion] = useState<number | null>(null)
  const [textError, setTextError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    void get<{ versions: SnapshotInfo[] | null }>(`/api/semantic/models/${modelId}/versions`).then(
      (res) => {
        setVersions(res?.versions ?? [])
      },
    )
  }, [open, modelId, get])

  const handleClose = useCallback(() => {
    setDiff(null)
    setFromVersion('')
    setToVersion('')
    setTextError(null)
    onClose()
  }, [onClose])

  // Version exports return raw YAML text — apiFetchText, not the JSON client.
  const fetchVersionYaml = useCallback(
    async (version: string) => {
      try {
        setTextError(null)
        return await apiFetchText(
          'GET',
          `/api/semantic/models/${modelId}/versions/${version}/export`,
        )
      } catch (err: unknown) {
        setTextError(err instanceof Error ? err.message : t('common.unknown_error'))
        return null
      }
    },
    [modelId, t],
  )

  const downloadVersion = useCallback(
    async (version: number) => {
      const yaml = await fetchVersionYaml(String(version))
      if (yaml != null) {
        downloadTextFile(`${modelName}-v${version}.yaml`, yaml)
      }
    },
    [fetchVersionYaml, modelName],
  )

  const rollbackTo = useCallback(
    async (version: number) => {
      const ok = await confirm({
        title: t('modeling.versions_restore_confirm', { version }),
        variant: 'danger',
        confirmLabel: t('modeling.versions_restore'),
      })
      if (!ok) {
        return
      }
      setBusyVersion(version)
      try {
        const res = await postData(`/api/semantic/models/${modelId}/rollback`, {
          version,
          published_by: 'modeling-ui',
        })
        if (res !== null) {
          onRolledBack(version)
          handleClose()
        }
      } finally {
        setBusyVersion(null)
      }
    },
    [confirm, t, postData, modelId, onRolledBack, handleClose],
  )

  const compare = useCallback(async () => {
    if (!fromVersion || !toVersion) {
      return
    }
    const [before, after] = await Promise.all([
      fetchVersionYaml(fromVersion),
      fetchVersionYaml(toVersion),
    ])
    if (before != null && after != null) {
      setDiff(diffLines(before, after))
    }
  }, [fromVersion, toVersion, fetchVersionYaml])

  const versionOptions = versions.map((v) => ({
    value: String(v.version),
    label: `v${v.version}`,
    hint: formatDateTime(v.created_at, languageTag),
  }))
  const latestVersion = versions.reduce((max, v) => Math.max(max, v.version), 0)

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={t('modeling.versions_title')}
      subtitle={modelName}
      className="max-w-3xl"
    >
      {(error ?? textError) && <p className="text-sm text-red-500">{error ?? textError}</p>}
      {versions.length === 0 && !loading && (
        <p className="text-muted-foreground text-sm">{t('modeling.versions_empty')}</p>
      )}
      {versions.length > 0 && (
        <div className="flex flex-col gap-5">
          <ol className="border-border divide-border m-0 flex max-h-64 list-none flex-col divide-y overflow-y-auto rounded-lg border p-0">
            {versions.map((v) => (
              <li key={v.version} className="flex items-center gap-3 px-3 py-2.5 text-sm">
                <span
                  className={cn(
                    'inline-flex h-7 min-w-11 items-center justify-center rounded-full px-2 text-[0.75rem] font-bold',
                    v.version === latestVersion
                      ? 'text-accent bg-[color-mix(in_srgb,var(--accent)_16%,transparent)]'
                      : 'bg-canvas-subtle text-foreground-muted',
                  )}
                >
                  v{v.version}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="text-foreground block leading-tight">
                    {formatDateTime(v.created_at, languageTag)}
                    {v.version === latestVersion && (
                      <span className="text-accent ml-2 text-[0.7rem] font-semibold">
                        {t('modeling.versions_current')}
                      </span>
                    )}
                  </span>
                  {v.created_by && (
                    <span className="text-foreground-muted block text-[0.75rem]">
                      {v.created_by}
                    </span>
                  )}
                </span>
                <button
                  type="button"
                  className={buttonClass('ghost', { size: 'sm', className: 'mt-0! w-auto!' })}
                  onClick={() => void downloadVersion(v.version)}
                >
                  ⬇ {t('modeling.versions_download')}
                </button>
                {v.version !== latestVersion && (
                  <button
                    type="button"
                    className={buttonClass('secondary', { size: 'sm', className: 'mt-0! w-auto!' })}
                    disabled={busyVersion !== null}
                    onClick={() => void rollbackTo(v.version)}
                  >
                    {busyVersion === v.version
                      ? t('modeling.versions_restoring')
                      : `↩ ${t('modeling.versions_restore')}`}
                  </button>
                )}
              </li>
            ))}
          </ol>
          {versions.length > 1 && (
            <div className="border-border border-t pt-4">
              <h4 className="text-foreground-muted m-0 mb-2 text-[0.7rem] font-bold tracking-widest uppercase">
                {t('modeling.versions_compare')}
              </h4>
              <div className="flex flex-wrap items-end gap-2">
                <div className="min-w-32 flex-1">
                  <label htmlFor="model-diff-from" className="mb-1 block text-xs font-medium">
                    {t('modeling.versions_diff_from')}
                  </label>
                  <Select
                    id="model-diff-from"
                    name="diff-from"
                    value={fromVersion}
                    onChange={setFromVersion}
                    placeholder={t('modeling.versions_pick')}
                    options={versionOptions}
                  />
                </div>
                <div className="min-w-32 flex-1">
                  <label htmlFor="model-diff-to" className="mb-1 block text-xs font-medium">
                    {t('modeling.versions_diff_to')}
                  </label>
                  <Select
                    id="model-diff-to"
                    name="diff-to"
                    value={toVersion}
                    onChange={setToVersion}
                    placeholder={t('modeling.versions_pick')}
                    options={versionOptions}
                  />
                </div>
                <button
                  type="button"
                  className={buttonClass('primary', { className: 'mt-0! w-auto!' })}
                  disabled={!fromVersion || !toVersion || fromVersion === toVersion || loading}
                  onClick={() => void compare()}
                >
                  {t('modeling.versions_compare')}
                </button>
              </div>
            </div>
          )}
          {diff && (
            <pre className="border-border bg-muted/30 max-h-80 overflow-auto rounded border p-3 text-xs leading-5">
              {diff.map((line, idx) => (
                <div key={idx} className={cn('whitespace-pre', diffLineClass(line.type))}>
                  {diffLinePrefix(line.type)}
                  {line.text}
                </div>
              ))}
            </pre>
          )}
        </div>
      )}
    </Modal>
  )
}
