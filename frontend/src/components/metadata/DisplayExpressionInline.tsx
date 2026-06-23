import { useState } from 'react'

import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import type { TableRow } from '../../types/semantic'

export function DisplayExpressionInline({
  table,
  onSave,
  t,
}: {
  table: TableRow
  onSave: (tab: TableRow, expr: string) => Promise<boolean>
  t: TFunction
}) {
  const original = table.display_expression ?? ''
  const [value, setValue] = useState(original)
  const [saving, setSaving] = useState(false)
  const dirty = value.trim() !== original.trim()

  const save = async () => {
    setSaving(true)
    await onSave(table, value.trim())
    setSaving(false)
  }

  return (
    <div className="flex items-center gap-1.5">
      <input
        type="text"
        className="border-border bg-canvas text-foreground focus-visible:border-accent/55 min-h-[1.85rem] w-full max-w-104 flex-1 rounded-[0.35rem] border px-2 py-[0.2rem] font-mono text-[0.72rem] outline-none"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && dirty && !saving) {
            void save()
          }
        }}
        placeholder={t('metadata.display_expr_placeholder')}
        spellCheck={false}
      />
      <button
        type="button"
        className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
        disabled={!dirty || saving}
        onClick={() => void save()}
      >
        {saving ? t('common.saving') : t('common.save')}
      </button>
    </div>
  )
}
