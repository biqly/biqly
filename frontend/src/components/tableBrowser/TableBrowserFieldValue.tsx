import { useEffect, useState } from 'react'

import type { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formatResultCell } from '../../utils/resultCellFormat'
import { TableBrowserCellValue } from './TableBrowserCellValue'
import {
  tableBrowserBoolDotFalseClass,
  tableBrowserBoolDotTrueClass,
  tableBrowserBoolFalseClass,
  tableBrowserBoolPillClass,
  tableBrowserBoolTrueClass,
  tableBrowserDetailEmptyClass,
  tableBrowserMonoValueClass,
  tableBrowserSecretActionsClass,
  tableBrowserSecretBtnClass,
  tableBrowserSecretMaskClass,
  tableBrowserSecretWrapClass,
} from './tableBrowserClasses'

type Translate = ReturnType<typeof useT>

// Column names whose values are secrets or opaque ciphertext — masked rather
// than dumped as an unreadable blob (dsn_encrypted, password_encrypted, api
// keys, tokens, …). Matches by name so it works for any table.
const SECRET_COLUMN_RE = /(password|secret|token|credential|_encrypted$|encrypted_|_key$|api_?key)/i

// Values best set in a monospace face so they line up and scan: surrogate
// keys, foreign keys, and UUID-shaped strings.
const ID_COLUMN_RE = /(^id$|_id$|uuid|hash)/i
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

function isSecretColumn(name: string): boolean {
  return SECRET_COLUMN_RE.test(name)
}

/** Interprets a raw cell as a boolean, tolerating the "true"/"false" strings
 * some drivers return for a boolean column. Returns null when it is not one. */
function asBoolean(raw: unknown): boolean | null {
  if (typeof raw === 'boolean') {
    return raw
  }
  if (typeof raw === 'string') {
    const t = raw.trim().toLowerCase()
    if (t === 'true') {
      return true
    }
    if (t === 'false') {
      return false
    }
  }
  return null
}

function looksMonospace(colName: string, display: string): boolean {
  return ID_COLUMN_RE.test(colName) || UUID_RE.test(display.trim())
}

/** Masked secret with reveal + copy actions (detail view) or a bare mask
 * (dense cell view, where there is no room for controls). */
function SecretValue({
  value,
  variant,
  t,
}: {
  value: string
  variant: 'cell' | 'detail'
  t: Translate
}) {
  const [revealed, setRevealed] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) {
      return
    }
    const id = setTimeout(() => setCopied(false), 1500)
    return () => clearTimeout(id)
  }, [copied])

  if (variant === 'cell') {
    return (
      <span className={tableBrowserSecretMaskClass} aria-label={t('table_browser.hidden_value')}>
        ••••••••
      </span>
    )
  }

  const copy = () => {
    void navigator.clipboard
      .writeText(value)
      .then(() => setCopied(true))
      .catch(() => undefined)
  }

  return (
    <span className={tableBrowserSecretWrapClass}>
      {revealed ? (
        <TableBrowserCellValue value={value} multiline className={tableBrowserMonoValueClass} />
      ) : (
        <span className={tableBrowserSecretMaskClass} aria-label={t('table_browser.hidden_value')}>
          ••••••••••••
        </span>
      )}
      <span className={tableBrowserSecretActionsClass}>
        <button
          type="button"
          className={tableBrowserSecretBtnClass}
          onClick={() => setRevealed((v) => !v)}
        >
          {revealed ? t('table_browser.hide') : t('table_browser.reveal')}
        </button>
        <button type="button" className={tableBrowserSecretBtnClass} onClick={copy}>
          {copied ? t('table_browser.copied') : t('table_browser.copy')}
        </button>
      </span>
    </span>
  )
}

/**
 * Renders one cell/field value by its type: booleans as status pills, secret
 * or encrypted columns masked (with reveal + copy in the detail view),
 * id/uuid-shaped values in monospace, and everything else through the shared
 * overflow-aware cell renderer. `variant` picks the density: `cell` for the
 * grid, `detail` for the row modal.
 */
export function TableBrowserFieldValue({
  colName,
  raw,
  variant,
  t,
  className,
}: {
  colName: string
  raw: unknown
  variant: 'cell' | 'detail'
  t: Translate
  className?: string
}) {
  const bool = asBoolean(raw)
  if (bool !== null) {
    return (
      <span
        className={cn(
          tableBrowserBoolPillClass,
          bool ? tableBrowserBoolTrueClass : tableBrowserBoolFalseClass,
        )}
      >
        <span
          className={bool ? tableBrowserBoolDotTrueClass : tableBrowserBoolDotFalseClass}
          aria-hidden="true"
        />
        {bool ? 'true' : 'false'}
      </span>
    )
  }

  const display = formatResultCell(raw, colName, {})
  if (!display) {
    return (
      <span className={tableBrowserDetailEmptyClass} aria-hidden="true">
        —
      </span>
    )
  }

  if (isSecretColumn(colName)) {
    return <SecretValue value={display} variant={variant} t={t} />
  }

  return (
    <TableBrowserCellValue
      value={display}
      multiline={variant === 'detail'}
      className={cn(looksMonospace(colName, display) && tableBrowserMonoValueClass, className)}
    />
  )
}
