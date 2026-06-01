import abiLogo from '../../assets/abi-logo.png'

interface LoadingIndicatorProps {
  label?: string
  size?: 'sm' | 'md'
  className?: string
}

export function LoadingIndicator({ label, size = 'md', className }: LoadingIndicatorProps) {
  return (
    <div
      className={[
        'loading-indicator',
        size === 'sm' ? 'loading-indicator--sm' : 'loading-indicator--md',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <div className="loading-indicator__mark" aria-hidden="true">
        <img src={abiLogo} alt="" className="loading-indicator__logo" draggable={false} />
        <span className="loading-indicator__ring" />
      </div>
      {label ? <span className="loading-indicator__label">{label}</span> : null}
    </div>
  )
}
