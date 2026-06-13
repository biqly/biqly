import type { DriverId } from '../dbDrivers'
import { DRIVER_IDS, driverLabelKey, driverLogoUrl } from '../dbDrivers'
import type { TranslationKey } from '../i18n'
import {
  driverTileClass,
  driverTileGridClass,
  driverTileLabelClass,
  driverTileLogoClass,
} from '../lib/driverClasses'

interface Props<T extends string = DriverId> {
  value: string
  onChange: (id: string) => void
  ariaLabel?: string
  ids?: readonly T[]
  t: (key: TranslationKey) => string
}

export function DriverTileGrid({ value, onChange, ariaLabel, ids = DRIVER_IDS, t }: Props) {
  return (
    <div className={driverTileGridClass()} role="radiogroup" aria-label={ariaLabel}>
      {(ids as string[]).map((id) => {
        const selected = value === id
        const labelKey = driverLabelKey(id)
        const label = t(labelKey)
        const logoSrc = driverLogoUrl(id)
        return (
          <button
            key={id}
            type="button"
            role="radio"
            aria-checked={selected}
            className={driverTileClass(id, selected)}
            onClick={() => onChange(id)}
          >
            <span className={driverTileLogoClass(id)} aria-hidden>
              <img src={logoSrc} alt="" width={40} height={40} />
            </span>
            <span className={driverTileLabelClass()}>{label}</span>
          </button>
        )
      })}
    </div>
  )
}
