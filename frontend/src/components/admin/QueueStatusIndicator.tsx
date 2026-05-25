import { useEffect, useState } from 'react'
import { getAIQueueStatus } from '../../api/admin'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import type { AIQueueStatus } from '../../types/auth'

interface Props {
  clientSessionID?: string
  pollMs?: number
}

export function QueueStatusIndicator({ clientSessionID, pollMs = 3000 }: Props) {
  const t = useT()
  const { accessToken } = useAuth()
  const [status, setStatus] = useState<AIQueueStatus | null>(null)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    if (!accessToken) return
    let cancelled = false

    async function tick() {
      try {
        const s = await getAIQueueStatus(accessToken as string, clientSessionID)
        if (!cancelled) {
          setStatus(s)
          setErr(null)
        }
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : String(e))
      }
    }

    tick()
    const interval = window.setInterval(tick, pollMs)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [accessToken, clientSessionID, pollMs])

  if (err) return null
  if (!status) return null
  if (status.total_pending === 0 && !status.my_job_id) return null

  return (
    <div
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 8,
        padding: '4px 10px',
        background: 'var(--surface-secondary, #f3f4f6)',
        borderRadius: 12,
        fontSize: 12,
        color: 'var(--text-secondary, #4b5563)',
      }}
      title={status.my_job_id ? `Job: ${status.my_job_id}` : t('admin.queue.not_queued')}
    >
      <span
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background:
            status.my_job_status === 'running'
              ? '#10b981'
              : status.my_job_status === 'queued' || status.my_job_status === 'pending'
                ? '#f59e0b'
                : '#9ca3af',
        }}
      />
      <span>
        {t('admin.queue.queue')}: <strong>{status.total_pending}</strong>
        {status.my_position && (
          <>
            {' · '}
            {t('admin.queue.position')}: <strong>#{status.my_position}</strong>
          </>
        )}
        {status.my_job_status === 'running' && (
          <>
            {' · '}{t('admin.queue.running')}
          </>
        )}
      </span>
    </div>
  )
}
