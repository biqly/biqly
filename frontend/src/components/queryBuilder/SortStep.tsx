import { useT } from '../../i18n'
import { Select } from '../ui/Select'
import { NotebookStep } from './NotebookStep'
import { qbTagBase, qbTagPurpleClass } from './queryBuilderClasses'

interface SortStepProps {
  orderBy: string
  orderDir: string
  orderByOpts: { value: string; label: string; hint?: string }[]
  setOrderBy: (v: string) => void
  setOrderDir: (v: string) => void
  onClear: () => void
}

export function SortStep({
  orderBy,
  orderDir,
  orderByOpts,
  setOrderBy,
  setOrderDir,
  onClear,
}: SortStepProps) {
  const t = useT()
  if (!orderBy) {
    return null
  }

  return (
    <NotebookStep label="Sort" themeClass="sort" onClose={onClear} closeTitle={t('common.cancel')}>
      <div className={`${qbTagBase} ${qbTagPurpleClass} flex items-center gap-1`}>
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
