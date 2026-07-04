import { useState } from 'react'

import { useT } from '../../i18n'
import {
  adminBadgeNeutralClass,
  adminBtnSecondaryClass,
  adminCardClass,
  adminLabelTextClass,
  adminValClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'

const codeBlockClass =
  'm-0 overflow-auto rounded-md bg-card-raised p-3 font-mono text-xs whitespace-pre-wrap wrap-break-word text-foreground'

const MCP_TOOLS = ['list_datasources', 'list_models', 'run_question', 'run_logical_query'] as const

function connectionSnippet(endpoint: string): string {
  return JSON.stringify(
    {
      mcpServers: {
        biqly: {
          type: 'http',
          url: endpoint,
          headers: { Authorization: 'Bearer <token>' },
        },
      },
    },
    null,
    2,
  )
}

export function MCPIntegrationPanel() {
  const t = useT()
  const [copied, setCopied] = useState(false)
  const endpoint = `${window.location.origin}/mcp`
  const snippet = connectionSnippet(endpoint)

  const copySnippet = () => {
    void navigator.clipboard.writeText(snippet).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <AdminPanelShell title={t('admin.mcp.title')} description={t('admin.mcp.description')}>
      <div className={adminCardClass}>
        <div className="flex flex-col gap-1">
          <span className={adminLabelTextClass}>{t('admin.mcp.endpoint')}</span>
          <span className={`${adminValClass} font-mono`}>{endpoint}</span>
        </div>
      </div>

      <div className={adminCardClass}>
        <div className="flex flex-col gap-2">
          <span className={adminLabelTextClass}>{t('admin.mcp.tools')}</span>
          <div className="flex flex-wrap gap-1">
            {MCP_TOOLS.map((tool) => (
              <span key={tool} className={adminBadgeNeutralClass}>
                {tool}
              </span>
            ))}
          </div>
          <p className="text-foreground-muted m-0 text-sm">{t('admin.mcp.governance_note')}</p>
        </div>
      </div>

      <div className={adminCardClass}>
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <span className={adminLabelTextClass}>{t('admin.mcp.snippet')}</span>
            <button type="button" className={adminBtnSecondaryClass} onClick={copySnippet}>
              {copied ? t('admin.mcp.copied') : t('admin.mcp.copy')}
            </button>
          </div>
          <pre className={codeBlockClass}>{snippet}</pre>
          <p className="text-foreground-muted m-0 text-sm">{t('admin.mcp.token_note')}</p>
        </div>
      </div>
    </AdminPanelShell>
  )
}
