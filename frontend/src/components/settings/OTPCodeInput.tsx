import { useAutofocus } from '../../hooks/useAutofocus'
import { useT } from '../../i18n'
import { legacyFormClass } from '../../lib/formClasses'
import { mfaOtpInputClass } from '../admin/adminClasses'
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
  const inputRef = useAutofocus<HTMLInputElement>(autoFocus)

  return (
    <div className={legacyFormClass('form-group')} style={{ margin: 0 }}>
      <label htmlFor={id}>{t('mfa.label_code')}</label>
      <input
        id={id}
        ref={inputRef}
        type="text"
        pattern="[0-9]*"
        inputMode="numeric"
        maxLength={6}
        required
        value={value}
        onChange={(event) => onChange(normalizeOTPCode(event.target.value))}
        placeholder={t('mfa.placeholder_code')}
        disabled={disabled}
        className={mfaOtpInputClass}
      />
    </div>
  )
}
