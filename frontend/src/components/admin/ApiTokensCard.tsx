import { useCallback, useState } from 'react'

import {
  type ApiToken,
  createApiToken,
  type CreatedApiToken,
  listApiTokens,
  revokeApiToken,
} from '../../api/apiTokens'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { useFetch } from '../../hooks/useFetch'
import { useT } from '../../i18n'
import { ErrorAlert } from '../ui/ErrorAlert'
import {
  adminBtnPrimaryClass,
  adminBtnRevokeClass,
  adminBtnSecondaryClass,
  adminCardClass,
  adminFormLabelClass,
  adminInputClass,
  adminLabelTextClass,
  adminSelectWideClass,
  adminTableClass,
  adminTableContainerClass,
  adminTableRowHoverClass,
  adminTdClass,
  adminTdMonoClass,
  adminTextMutedClass,
  adminThClass,
  adminTheadRowClass,
} from './adminClasses'

const codeBlockClass =
  'm-0 overflow-auto rounded-md bg-card-raised p-3 font-mono text-xs whitespace-pre-wrap wrap-break-word text-foreground'

type ExpiryOption = '30' | '90' | '365' | 'never'

function expiresInDaysFromOption(option: ExpiryOption): number | undefined {
  return option === 'never' ? undefined : Number(option)
}

function formatDate(value: string | null | undefined): string | null {
  return value ? new Date(value).toLocaleDateString() : null
}

// ApiTokensCard lets a user generate, list, and revoke personal access
// tokens for programmatic/MCP access — a long-lived alternative to pasting
// their short-lived session JWT into the connection snippet below it.
export function ApiTokensCard() {
  const t = useT()
  const confirmMutation = useConfirmedMutation()
  const create = useAsyncState({ useSaving: true })
  const [reloadKey, setReloadKey] = useState(0)
  const [name, setName] = useState('')
  const [expiry, setExpiry] = useState<ExpiryOption>('90')
  const [revealed, setRevealed] = useState<CreatedApiToken | null>(null)
  const [copied, setCopied] = useState(false)

  const {
    data: tokens,
    loading,
    error: listError,
  } = useFetch<ApiToken[]>(() => listApiTokens(), [reloadKey])

  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  const handleCreate = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      create.setError(t('admin.mcp.tokens.name_required'))
      return
    }
    const created = await create.run(() =>
      createApiToken({ name: trimmed, expiresInDays: expiresInDaysFromOption(expiry) }),
    )
    if (created) {
      setRevealed(created)
      setName('')
      reload()
    }
  }

  const handleRevoke = async (id: string) => {
    const ok = await confirmMutation(() => revokeApiToken(id), {
      title: t('admin.mcp.tokens.revoke_confirm'),
      successMessage: t('admin.mcp.tokens.revoke_success'),
    })
    if (ok) {
      reload()
    }
  }

  const copyRevealed = () => {
    if (!revealed) {
      return
    }
    void navigator.clipboard.writeText(revealed.token).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className={adminCardClass}>
      <div className="flex flex-col gap-3">
        <span className={adminLabelTextClass}>{t('admin.mcp.tokens.title')}</span>

        {revealed && (
          <div className="border-border bg-card-raised flex flex-col gap-2 rounded-md border p-3">
            <p className="text-warning m-0 text-sm font-medium">
              {t('admin.mcp.tokens.reveal_warning')}
            </p>
            <div className="flex items-center gap-2">
              <pre className={`${codeBlockClass} flex-1`}>{revealed.token}</pre>
              <button type="button" className={adminBtnSecondaryClass} onClick={copyRevealed}>
                {copied ? t('admin.mcp.copied') : t('admin.mcp.copy')}
              </button>
            </div>
            <p className="text-foreground-muted m-0 text-sm">
              {t('admin.mcp.tokens.reveal_usage_note')}
            </p>
            <button
              type="button"
              className={`${adminBtnPrimaryClass} self-start`}
              onClick={() => setRevealed(null)}
            >
              {t('common.confirm_ok')}
            </button>
          </div>
        )}

        <ErrorAlert error={listError} />

        {loading && <p className={adminTextMutedClass}>{t('common.loading')}</p>}

        {!loading && tokens?.length === 0 && (
          <p className={adminTextMutedClass}>{t('admin.mcp.tokens.empty_state')}</p>
        )}

        {!loading && tokens && tokens.length > 0 && (
          <div className={adminTableContainerClass}>
            <table className={adminTableClass}>
              <thead>
                <tr className={adminTheadRowClass}>
                  <th className={adminThClass}>{t('admin.mcp.tokens.column_name')}</th>
                  <th className={adminThClass}>{t('admin.mcp.tokens.column_prefix')}</th>
                  <th className={adminThClass}>{t('admin.mcp.tokens.column_created')}</th>
                  <th className={adminThClass}>{t('admin.mcp.tokens.column_last_used')}</th>
                  <th className={adminThClass}>{t('admin.mcp.tokens.column_expires')}</th>
                  <th className={adminThClass} />
                </tr>
              </thead>
              <tbody>
                {tokens.map((token) => (
                  <tr key={token.id} className={adminTableRowHoverClass}>
                    <td className={adminTdClass}>{token.name}</td>
                    <td className={adminTdMonoClass}>{token.token_prefix}****</td>
                    <td className={adminTdClass}>{formatDate(token.created_at)}</td>
                    <td className={adminTdClass}>
                      {formatDate(token.last_used_at) ?? t('admin.mcp.tokens.last_used_never')}
                    </td>
                    <td className={adminTdClass}>
                      {formatDate(token.expires_at) ?? t('admin.mcp.tokens.never')}
                    </td>
                    <td className={adminTdClass}>
                      <button
                        type="button"
                        className={adminBtnRevokeClass}
                        onClick={() => void handleRevoke(token.id)}
                      >
                        {t('admin.mcp.tokens.revoke')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <ErrorAlert error={create.error} />

        <div className="flex flex-wrap items-end gap-3">
          <label className={`${adminFormLabelClass} flex-1`}>
            {t('admin.mcp.tokens.name_label')}
            <input
              type="text"
              className={adminInputClass}
              value={name}
              placeholder={t('admin.mcp.tokens.name_placeholder')}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className={adminFormLabelClass}>
            {t('admin.mcp.tokens.expiry_label')}
            <select
              className={adminSelectWideClass}
              value={expiry}
              onChange={(e) => setExpiry(e.target.value as ExpiryOption)}
            >
              <option value="30">{t('admin.mcp.tokens.expiry_30_days')}</option>
              <option value="90">{t('admin.mcp.tokens.expiry_90_days')}</option>
              <option value="365">{t('admin.mcp.tokens.expiry_1_year')}</option>
              <option value="never">{t('admin.mcp.tokens.expiry_never')}</option>
            </select>
          </label>
          <button
            type="button"
            className={adminBtnPrimaryClass}
            disabled={create.saving}
            onClick={() => void handleCreate()}
          >
            {create.saving ? t('admin.mcp.tokens.creating') : t('admin.mcp.tokens.create_button')}
          </button>
        </div>
      </div>
    </div>
  )
}
