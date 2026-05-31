import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'

interface LoadingScreenProps {
  label?: string
  minHeight?: string
}

export function LoadingScreen({ label, minHeight = '60vh' }: LoadingScreenProps) {
  const t = useT()
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        flex: 1,
        height: '100%',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight,
        gap: '20px',
      }}
      role="status"
      aria-live="polite"
    >
      <div className="loading-logo-container">
        <img
          src={abiLogo}
          alt="ABI Logo"
          className="loading-logo-image"
        />
        <div className="loading-logo-spinner" />
      </div>
      <span style={{ color: 'var(--text-secondary, #a1a1aa)', fontSize: '14px', fontWeight: 500 }}>
        {label ?? t('common.loading')}
      </span>
    </div>
  )
}
