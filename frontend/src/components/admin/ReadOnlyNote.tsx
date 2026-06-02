import { useT } from '../../i18n'

// ReadOnlyNote renders a consistent banner explaining that the current user can
// view but not modify the panel. Shown when the caller lacks the permission the
// backend requires for the panel's mutations (the backend is the real gate;
// this is a UX affordance so controls are not silently rejected).
export function ReadOnlyNote() {
  const t = useT()
  return (
    <p className="form-hint form-hint--warning" role="note" style={{ marginBottom: 12 }}>
      {t('admin.read_only_note')}
    </p>
  )
}
