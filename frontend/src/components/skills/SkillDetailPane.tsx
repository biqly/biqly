import type { TFunction } from '../../i18n'
import { tagPillClass, tagPillMetaClass } from '../../lib/badgeClasses'
import { buttonClass } from '../../lib/buttonClasses'
import { legacyFormClass } from '../../lib/formClasses'
import {
  savedQuestionActionsClass,
  savedQuestionDescriptionClass,
} from '../../lib/savedQuestionClasses'
import { ResultTable } from '../ResultTable'
import { EmptyState } from '../ui/EmptyState'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import type { Skill, SkillRunResult } from './types'
import { paramDefaultText } from './types'

interface SkillDetailPaneProps {
  skill: Skill | null
  paramValues: Record<string, string>
  onParamChange: (name: string, value: string) => void
  runLoading: boolean
  runError: string | null
  runResult: SkillRunResult | null
  onRun: () => void
  onOpenEdit: (skill: Skill) => void
  onDelete: (id: string) => void
  t: TFunction
}

export function SkillDetailPane({
  skill,
  paramValues,
  onParamChange,
  runLoading,
  runError,
  runResult,
  onRun,
  onOpenEdit,
  onDelete,
  t,
}: SkillDetailPaneProps) {
  if (!skill) {
    return <EmptyState description={t('skills.select_hint')} />
  }

  return (
    <div>
      <h2>{skill.name}</h2>
      {skill.description && (
        <p className={`${savedQuestionDescriptionClass()} mt-2`}>{skill.description}</p>
      )}

      <div className="my-4 flex flex-wrap gap-2">
        <span className={tagPillMetaClass}>
          <strong>{t('skills.version')}:</strong> <code>v{skill.version}</code>
        </span>
        <span className={tagPillMetaClass}>
          <strong>{t('skills.last_verified')}:</strong>{' '}
          <code>
            {skill.last_verified_at
              ? new Date(skill.last_verified_at).toLocaleString()
              : t('skills.never_verified')}
          </code>
        </span>
        {skill.tags.map((tag) => (
          <span key={tag} className={tagPillClass}>
            {tag}
          </span>
        ))}
      </div>

      {skill.parameters.length > 0 && (
        <div className="mb-4">
          <h3>{t('skills.params_heading')}</h3>
          {skill.parameters.map((param) => (
            <div key={param.name} className={legacyFormClass('form-group')}>
              <label htmlFor={`skill-param-${param.name}`}>
                {param.label ?? param.name}
                {param.required ? ' *' : ''}
              </label>
              <input
                id={`skill-param-${param.name}`}
                value={paramValues[param.name] ?? ''}
                onChange={(e) => onParamChange(param.name, e.target.value)}
                placeholder={paramDefaultText(param.default)}
                autoComplete="off"
              />
            </div>
          ))}
        </div>
      )}

      <div className={savedQuestionActionsClass()}>
        <button
          type="button"
          className={buttonClass('primary', { size: 'sm' })}
          onClick={onRun}
          disabled={runLoading}
          aria-label={t('skills.aria_run')}
        >
          {runLoading ? t('skills.running') : t('skills.run')}
        </button>
        <button
          type="button"
          className={buttonClass('ghost', { size: 'sm' })}
          onClick={() => onOpenEdit(skill)}
          aria-label={t('skills.aria_edit')}
        >
          {t('skills.edit')}
        </button>
        <button
          type="button"
          className={buttonClass('danger', { size: 'sm' })}
          onClick={() => onDelete(skill.id)}
          aria-label={t('skills.aria_delete')}
        >
          {t('skills.delete')}
        </button>
      </div>

      <div className="relative mt-4">
        <LoadingOverlay loading={runLoading} />
        {runError && <ErrorAlert error={runError} />}
        {runResult?.result && (
          <>
            {runResult.sql && (
              <details className="mb-3">
                <summary>{t('skills.sql_heading')}</summary>
                <pre className="overflow-x-auto text-xs">{runResult.sql}</pre>
              </details>
            )}
            <ResultTable
              columns={runResult.result.columns}
              rows={runResult.result.rows}
              rowCount={runResult.result.rows.length}
              durationMs={runResult.result.stats?.duration_ms}
            />
          </>
        )}
      </div>
    </div>
  )
}
