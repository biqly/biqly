import { useT } from '../../i18n'
import { normalizeOTPCode } from './otp'

interface OTPCodeInputProps {
  id: string
  value: string
  onChange: (value: string) => void
  disabled: boolean
  autoFocus?: boolean
}

export function OTPCodeInput({
  id,
  value,
  onChange,
  disabled,
  autoFocus = false,
}: OTPCodeInputProps) {
  const t = useT()

  return (
    <div className="form-group" style={{ margin: 0 }}>
      <label htmlFor={id}>{t('mfa.label_code')}</label>
      <input
        id={id}
        type="text"
        pattern="[0-9]*"
        inputMode="numeric"
        maxLength={6}
        required
        value={value}
        onChange={(event) => onChange(normalizeOTPCode(event.target.value))}
        placeholder={t('mfa.placeholder_code')}
        disabled={disabled}
        autoFocus={autoFocus}
        className="mfa-otp-input"
      />
    </div>
  )
}
