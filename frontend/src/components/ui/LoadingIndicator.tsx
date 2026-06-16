import abiLogo from '../../assets/abi-logo.png'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'

interface LoadingIndicatorProps {
  label?: string
  size?: 'sm' | 'md'
  className?: string
  style?: React.CSSProperties
}

export function LoadingIndicator({ label, size = 'md', className, style }: LoadingIndicatorProps) {
  return (
    <div
      style={style}
      className={cn(
        legacyCardClass(
          "[data-theme='light']_&]:bg-[color-mix(in_srgb,var(--bg-card)_92%,transparent)] pointer-events-none inline-flex max-w-[min(18rem,calc(100vw-2rem))] items-center gap-[0.65rem] rounded-full border border-border bg-[color-mix(in_srgb,var(--bg-card)_88%,transparent)] p-[0.4rem_0.85rem_0.4rem_0.4rem] shadow-[0_8px_24px_rgba(0,0,0,0.28)] backdrop-blur-[10px]",
        ),
        className ?? '',
      )}
    >
      <div
        className={`relative shrink-0 ${size === 'sm' ? 'w-7 h-7' : 'w-9 h-9'}`}
        aria-hidden="true"
      >
        <img
          src={abiLogo}
          alt=""
          className="relative z-1 h-full w-full animate-[loading-logo-breathe_2.4s_ease-in-out_infinite] object-contain opacity-[0.52] motion-reduce:animate-none motion-reduce:opacity-[0.5]"
          draggable={false}
        />
        <span className="absolute inset-[-3px] animate-[loading-spin_0.85s_linear_infinite] rounded-full border-2 border-transparent border-t-accent opacity-[0.85] motion-reduce:animate-none" />
      </div>
      {label ? (
        <span className="overflow-hidden pr-[0.15rem] text-[0.8125rem] leading-[1.2] font-medium text-ellipsis whitespace-nowrap text-foreground-muted">
          {label}
        </span>
      ) : null}
    </div>
  )
}
