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
          "inline-flex items-center gap-[0.65rem] max-w-[min(18rem,calc(100vw-2rem))] p-[0.4rem_0.85rem_0.4rem_0.4rem] rounded-full border border-border bg-[color-mix(in_srgb,var(--bg-card)_88%,transparent)] [data-theme='light']_&]:bg-[color-mix(in_srgb,var(--bg-card)_92%,transparent)] shadow-[0_8px_24px_rgba(0,0,0,0.28)] backdrop-blur-[10px] pointer-events-none",
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
          className="relative z-[1] w-full h-full object-contain opacity-[0.52] animate-[loading-logo-breathe_2.4s_ease-in-out_infinite] motion-reduce:animate-none motion-reduce:opacity-[0.5]"
          draggable={false}
        />
        <span className="absolute -inset-[3px] border-2 border-transparent border-t-accent rounded-full opacity-[0.85] animate-[loading-spin_0.85s_linear_infinite] motion-reduce:animate-none" />
      </div>
      {label ? (
        <span className="pr-[0.15rem] text-foreground-muted text-[0.8125rem] font-medium leading-[1.2] whitespace-nowrap overflow-hidden text-ellipsis">
          {label}
        </span>
      ) : null}
    </div>
  )
}
