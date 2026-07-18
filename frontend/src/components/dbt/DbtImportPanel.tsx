import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { type DbtImportResult, importDbtProject } from '../../api/dbt'
import { useDatasources } from '../../hooks/useDatasources'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { formLabelClass } from '../../lib/formClasses'
import { ErrorAlert } from '../ui/ErrorAlert'
import { Select } from '../ui/Select'
import { dbtManifestModelNames } from './dbtManifestPreview'

// Native file inputs restyled to match the card language: bordered field with
// a button-like trigger. Keeps the native control for accessibility.
const dbtFileInputClass = cn(
  'border-border bg-card-raised text-foreground-muted w-full cursor-pointer rounded-md border text-sm',
  'file:bg-surface-hover file:text-foreground file:mr-3 file:cursor-pointer file:rounded-l-[5px]',
  'file:border-0 file:px-3 file:py-2.5 file:text-sm file:font-medium',
)

export function DbtImportPanel() {
  const t = useT()
  const navigate = useNavigate()
  const { datasources, loading: datasourcesLoading } = useDatasources()
  const [datasourceId, setDatasourceId] = useState('')
  const [manifest, setManifest] = useState<File | null>(null)
  const [catalog, setCatalog] = useState<File | null>(null)
  const [preview, setPreview] = useState<string[]>([])
  const [previewError, setPreviewError] = useState<string | null>(null)
  const [result, setResult] = useState<DbtImportResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [importing, setImporting] = useState(false)

  const datasourceOptions = useMemo(
    () => datasources.map((ds) => ({ value: ds.id, label: `${ds.name} · ${ds.type}` })),
    [datasources],
  )

  const handleManifestChange = (file: File | null) => {
    setManifest(file)
    setResult(null)
    setError(null)
    setPreview([])
    setPreviewError(null)
    if (!file) {
      return
    }
    void previewManifest(file, setPreview, setPreviewError, t('dbt_import.manifest_read_error'))
  }

  const handleImport = async () => {
    if (!manifest || !datasourceId || importing) {
      return
    }
    setImporting(true)
    setError(null)
    setResult(null)
    try {
      setResult(await importDbtProject({ datasourceId, manifest, catalog }))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('common.unknown_error'))
    } finally {
      setImporting(false)
    }
  }

  return (
    <section
      className={cn(cardClass({ elevated: true }), 'mb-0 flex flex-col gap-4')}
      aria-labelledby="dbt-import-heading"
    >
      <div>
        <h2 id="dbt-import-heading" className="m-0">
          {t('dbt_import.title')}
        </h2>
        <p className="text-foreground-muted mt-1 mb-0 text-sm leading-[1.45]">
          {t('dbt_import.hint')}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 min-[900px]:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <label className={formLabelClass} htmlFor="dbt-import-datasource">
            {t('dbt_import.datasource_label')}
          </label>
          <Select
            id="dbt-import-datasource"
            value={datasourceId}
            onChange={setDatasourceId}
            options={datasourceOptions}
            searchable
            placeholder={t('dbt_import.datasource_placeholder')}
            disabled={datasourcesLoading}
            ariaLabel={t('dbt_import.datasource_label')}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className={formLabelClass} htmlFor="dbt-import-manifest">
            {t('dbt_import.manifest_label')}
          </label>
          <input
            id="dbt-import-manifest"
            type="file"
            accept="application/json,.json"
            className={dbtFileInputClass}
            onChange={(event) => handleManifestChange(event.target.files?.[0] ?? null)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className={formLabelClass} htmlFor="dbt-import-catalog">
            {t('dbt_import.catalog_label')}
          </label>
          <input
            id="dbt-import-catalog"
            type="file"
            accept="application/json,.json"
            className={dbtFileInputClass}
            onChange={(event) => setCatalog(event.target.files?.[0] ?? null)}
          />
          <span className="text-foreground-muted text-xs">{t('dbt_import.catalog_hint')}</span>
        </div>
      </div>

      {previewError && <ErrorAlert error={previewError} />}
      {manifest && !previewError && (
        <div className="border-border bg-card-raised rounded-md border p-3" aria-live="polite">
          <div className="mb-2 flex items-center justify-between gap-2">
            <strong className="text-sm">{t('dbt_import.preview_title')}</strong>
            <span className="text-foreground-muted text-xs">
              {t('dbt_import.preview_count', { count: preview.length })}
            </span>
          </div>
          {preview.length ? (
            <ul
              className="m-0 flex flex-wrap gap-1.5 p-0"
              aria-label={t('dbt_import.preview_title')}
            >
              {preview.map((name) => (
                <li className="bg-surface-hover list-none rounded px-2 py-1 text-xs" key={name}>
                  {name}
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-foreground-muted m-0 text-sm">{t('dbt_import.preview_empty')}</p>
          )}
        </div>
      )}

      {error && <ErrorAlert error={error} />}
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          className={buttonClass('primary', { className: 'mt-0! w-auto!' })}
          disabled={!manifest || !datasourceId || importing}
          onClick={() => void handleImport()}
        >
          {importing ? t('dbt_import.importing') : t('dbt_import.import_action')}
        </button>
      </div>

      {result && (
        <ImportResult
          result={result}
          onOpenModeling={() =>
            void navigate(`/modeling?datasource_id=${encodeURIComponent(datasourceId)}`)
          }
        />
      )}
    </section>
  )
}

function ImportResult({
  result,
  onOpenModeling,
}: {
  result: DbtImportResult
  onOpenModeling: () => void
}) {
  const t = useT()
  const importedNames = result.imported_models.map(({ model }) => model.name)
  return (
    <div className="border-success/40 bg-success/8 rounded-md border p-3" aria-live="polite">
      <strong className="text-sm">{t('dbt_import.result_title')}</strong>
      <ResultList title={t('dbt_import.imported_models')} values={importedNames} />
      <ResultList title={t('dbt_import.skipped_models')} values={result.skipped} />
      <ResultList title={t('dbt_import.warnings')} values={result.warnings} />
      <button
        type="button"
        className={buttonClass('secondary', { className: 'mt-3! w-auto!' })}
        onClick={onOpenModeling}
      >
        {t('dbt_import.open_modeling')}
      </button>
    </div>
  )
}

function ResultList({ title, values }: { title: string; values: string[] }) {
  if (!values.length) {
    return null
  }
  return (
    <div className="mt-3">
      <strong className="text-xs">{title}</strong>
      <ul className="mt-1 mb-0 pl-5 text-sm">
        {values.map((value) => (
          <li key={value}>{value}</li>
        ))}
      </ul>
    </div>
  )
}

async function previewManifest(
  file: File,
  setPreview: (names: string[]) => void,
  setPreviewError: (message: string | null) => void,
  errorMessage: string,
): Promise<void> {
  try {
    const contents = await readFile(file)
    const manifest: unknown = JSON.parse(contents)
    setPreview(dbtManifestModelNames(manifest))
  } catch {
    setPreviewError(errorMessage)
  }
}

function readFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error ?? new Error('Could not read file.'))
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
    reader.readAsText(file)
  })
}
