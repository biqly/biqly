import { cn } from './cn'

export function driverTileGridClass(): string {
  return cn('mt-[0.55rem] grid w-full grid-cols-[repeat(auto-fill,minmax(7rem,1fr))] gap-[0.65rem]')
}

export function driverTileClass(_driverId: string, selected: boolean): string {
  return cn(
    'm-0 flex cursor-pointer flex-col items-center gap-[0.4rem] rounded-[9px] border border-border bg-[var(--bg-card-alt)] p-[0.55rem_0.35rem] text-[0.72rem] font-semibold text-foreground transition-[border-color,box-shadow,background] duration-150',
    'hover:border-[var(--border-strong)] hover:bg-card',
    'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--accent)]',
    selected &&
      'border-[var(--accent-strong)] bg-[rgba(59,130,246,0.12)] shadow-[inset_0_0_0_1px_rgba(59,130,246,0.2)] dark:bg-[rgba(59,130,246,0.16)]',
  )
}

export function driverTileLogoClass(driverId: string): string {
  return cn(
    'flex h-[2.85rem] w-[2.85rem] items-center justify-center overflow-hidden rounded-lg bg-white shadow-[0_2px_6px_rgba(0,0,0,0.12)] dark:shadow-[0_2px_8px_rgba(0,0,0,0.45)]',
    driverId === 'clickhouse' && 'bg-[#ffcc01]',
    '[&_img]:h-[calc(100%-0.5rem)] [&_img]:w-[calc(100%-0.5rem)] [&_img]:object-contain',
    driverId === 'mysql' && '[&_img]:h-[calc(100%-0.35rem)] [&_img]:w-[calc(100%-0.35rem)]',
  )
}

export function driverTileLabelClass(): string {
  return 'text-center leading-[1.2]'
}

export function driverCellClass(): string {
  return 'inline-flex items-center gap-2 align-middle'
}

export function driverCellLogoClass(driverId: string): string {
  return cn(
    'flex h-[1.95rem] w-[1.95rem] shrink-0 items-center justify-center overflow-hidden rounded-md bg-white shadow-[0_1px_4px_rgba(0,0,0,0.1)] dark:shadow-[0_1px_5px_rgba(0,0,0,0.4)]',
    driverId === 'clickhouse' && 'bg-[#ffcc01]',
    '[&_img]:h-[calc(100%-0.3rem)] [&_img]:w-[calc(100%-0.3rem)] [&_img]:object-contain',
  )
}

export function driverCellLabelClass(): string {
  return 'text-[0.88rem] font-semibold text-foreground'
}
