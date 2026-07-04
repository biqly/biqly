import { useCallback, useEffect, useState } from 'react'

import { useApi } from '../../hooks/useApi'
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
}: {
  open: boolean
  modelId: string
  modelName: string
  onClose: () => void
}) {
  const t = useT()
  const [locale] = useLocale()
  const languageTag = localeLanguageTag(locale)
  const { get, loading, error } = useApi()
  const [versions, setVersions] = useState<SnapshotInfo[]>([])
  const [fromVersion, setFromVersion] = useState('')
  const [toVersion, setToVersion] = useState('')
  const [diff, setDiff] = useState<DiffLine[] | null>(null)

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
    onClose()
  }, [onClose])

  const fetchVersionYaml = useCallback(
    (version: string) => get<string>(`/api/semantic/models/${modelId}/versions/${version}/export`),
    [get, modelId],
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

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={t('modeling.versions_title')}
      subtitle={modelName}
      className="max-w-3xl"
    >
      {error && <p className="text-sm text-red-500">{error}</p>}
      {versions.length === 0 && !loading && (
        <p className="text-muted-foreground text-sm">{t('modeling.versions_empty')}</p>
      )}
      {versions.length > 0 && (
        <div className="flex flex-col gap-4">
          <ul className="flex max-h-48 flex-col gap-1 overflow-y-auto">
            {versions.map((v) => (
              <li
                key={v.version}
                className="border-border flex items-center justify-between gap-2 rounded border px-3 py-2 text-sm"
              >
                <span>
                  <strong>v{v.version}</strong>
                  <span className="text-muted-foreground ml-2">
                    {formatDateTime(v.created_at, languageTag)}
                    {v.created_by ? ` · ${v.created_by}` : ''}
                  </span>
                </span>
                <button
                  type="button"
                  className={buttonClass('secondary', { className: 'mt-0! w-auto!' })}
                  onClick={() => void downloadVersion(v.version)}
                >
                  {t('modeling.versions_download')}
                </button>
              </li>
            ))}
          </ul>
          {versions.length > 1 && (
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
