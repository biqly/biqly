import type { TFunction } from '../../i18n'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'

interface SortStepProps {
  orderBy: string
  orderDir: string
  orderByOpts: { value: string; label: string; hint?: string }[]
  setOrderBy: (v: string) => void
  setOrderDir: (v: string) => void
  onClear: () => void
  t: TFunction
}

export function SortStep({
  orderBy,
  orderDir,
  orderByOpts,
  setOrderBy,
  setOrderDir,
  onClear,
  t,
}: SortStepProps) {
  if (!orderBy) {
    return null
  }

  return (
    <NotebookStep label="Sort" themeClass="sort" onClose={onClear} closeTitle={t('common.cancel')}>
      <div
        className="notebook-tag notebook-tag--purple"
        style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}
      >
        <Select
          value={orderBy}
          onChange={setOrderBy}
          placeholder={t('query_builder.pick_field_placeholder')}
          options={orderByOpts}
          size="sm"
        />
        <Select
          value={orderDir}
          onChange={setOrderDir}
          options={[
            { value: 'asc', label: 'ASC' },
            { value: 'desc', label: 'DESC' },
          ]}
          size="sm"
        />
      </div>
    </NotebookStep>
  )
}
