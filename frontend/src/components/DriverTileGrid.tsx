import type { DriverId } from '../dbDrivers'
import { DRIVER_IDS, driverLabelKey, driverLogoUrl } from '../dbDrivers'
import type { TranslationKey } from '../i18n'

interface Props<T extends string = DriverId> {
  value: string
  onChange: (id: string) => void
  ariaLabel?: string
  ids?: readonly T[]
  t: (key: TranslationKey) => string
}

export function DriverTileGrid({ value, onChange, ariaLabel, ids = DRIVER_IDS, t }: Props) {
  return (
    <div className="driver-tile-grid" role="radiogroup" aria-label={ariaLabel}>
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
            className={`driver-tile driver-tile--${id} ${selected ? 'driver-tile--selected' : ''}`}
            onClick={() => onChange(id)}
          >
            <span className="driver-tile__logo" aria-hidden>
              <img src={logoSrc} alt="" width={40} height={40} />
            </span>
            <span className="driver-tile__label">{label}</span>
          </button>
        )
      })}
    </div>
  )
}
