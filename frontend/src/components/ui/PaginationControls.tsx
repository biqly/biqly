const btnActive: React.CSSProperties = {
  padding: '6px 12px',
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.08))',
  color: 'var(--text-primary, #f4f4f5)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
  transition: 'all 150ms',
}

const btnDisabled: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: 'var(--text-muted, #8a8a92)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  cursor: 'not-allowed',
  fontSize: 13,
  fontWeight: 500,
  opacity: 0.5,
}

export function PaginationControls({
  currentPage,
  safeTotalPages,
  singlePage,
  onPageChange,
  prevLabel,
  nextLabel,
  firstTitle,
  lastTitle,
}: {
  currentPage: number
  safeTotalPages: number
  singlePage: boolean
  onPageChange: (page: number) => void
  prevLabel: string
  nextLabel: string
  firstTitle: string
  lastTitle: string
}) {
  const atStart = singlePage || currentPage === 1
  const atEnd = singlePage || currentPage === safeTotalPages

  return (
    <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
      <button
        type="button"
        onClick={() => onPageChange(1)}
        disabled={atStart}
        style={atStart ? btnDisabled : btnActive}
        title={firstTitle}
      >
        «
      </button>
      <button
        type="button"
        onClick={() => onPageChange(currentPage - 1)}
        disabled={atStart}
        style={atStart ? btnDisabled : btnActive}
      >
        {prevLabel}
      </button>
      <span style={{ fontSize: 13, color: 'var(--text-primary, #f4f4f5)', margin: '0 8px' }}>
        {currentPage} / {safeTotalPages}
      </span>
      <button
        type="button"
        onClick={() => onPageChange(currentPage + 1)}
        disabled={atEnd}
        style={atEnd ? btnDisabled : btnActive}
      >
        {nextLabel}
      </button>
      <button
        type="button"
        onClick={() => onPageChange(safeTotalPages)}
        disabled={atEnd}
        style={atEnd ? btnDisabled : btnActive}
        title={lastTitle}
      >
        »
      </button>
    </div>
  )
}
