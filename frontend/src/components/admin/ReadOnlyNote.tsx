import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formHintWarningClass } from '../../lib/formClasses'

// ReadOnlyNote renders a consistent banner explaining that the current user can
// view but not modify the panel. Shown when the caller lacks the permission the
// backend requires for the panel's mutations (the backend is the real gate;
// this is a UX affordance so controls are not silently rejected).
export function ReadOnlyNote() {
  const t = useT()
  return (
    <p className={cn(formHintWarningClass, 'mb-3')} role="note">
      {t('admin.read_only_note')}
    </p>
  )
}
