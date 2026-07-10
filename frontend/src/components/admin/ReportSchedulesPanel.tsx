import { useCallback, useEffect, useState } from 'react'

import { useAdminApi } from '../../hooks/useApi'
import { useT } from '../../i18n'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { Button } from '../ui/Button'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import { EmptyState } from '../ui/EmptyState'
import { FormField } from '../ui/FormField'
import { LoadingScreen } from '../ui/LoadingScreen'
import { MultiSelect } from '../ui/MultiSelect'
import { Select } from '../ui/Select'
import {
  adminTableClass,
  adminTableContainerClass,
  adminTdClass,
  adminThClass,
  adminTheadRowClass,
  adminTrClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'

interface ReportSchedule {
  id: string
  name: string
  skill_ids: string[]
  recipients: string[]
  cadence: 'daily' | 'weekly' | 'monthly'
  hour_utc: number
  weekday: number
  day_of_month: number
  is_active: boolean
  last_run_at?: string
  last_status: string
  last_error: string
}

interface SkillOption {
  id: string
  name: string
}

interface FormState {
  id: string | null
  name: string
  skillIDs: string[]
  recipients: string
  cadence: 'daily' | 'weekly' | 'monthly'
  hourUTC: string
  weekday: string
  dayOfMonth: string
  isActive: boolean
}

const emptyForm: FormState = {
  id: null,
  name: '',
  skillIDs: [],
  recipients: '',
  cadence: 'daily',
  hourUTC: '7',
  weekday: '1',
  dayOfMonth: '1',
  isActive: true,
}

function scheduleToForm(s: ReportSchedule): FormState {
  return {
    id: s.id,
    name: s.name,
    skillIDs: s.skill_ids,
    recipients: s.recipients.join(', '),
    cadence: s.cadence,
    hourUTC: String(s.hour_utc),
    weekday: String(s.weekday),
    dayOfMonth: String(s.day_of_month),
    isActive: s.is_active,
  }
}

function ReportScheduleEmptyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="40"
      height="40"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <rect x="3" y="4" width="18" height="17" rx="2" />
      <path d="M3 9h18" />
      <path d="M8 2v4" />
      <path d="M16 2v4" />
      <path d="M8 13h3" />
      <path d="M8 17h6" />
    </svg>
  )
}

function ReportSchedulesPanel() {
  const t = useT()
  const { get, postData, putData, deleteData, loading, error } = useAdminApi()
  const [schedules, setSchedules] = useState<ReportSchedule[]>([])
  const [skills, setSkills] = useState<SkillOption[]>([])
  const [loaded, setLoaded] = useState(false)
  const [form, setForm] = useState<FormState | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ReportSchedule | null>(null)
  const [saving, setSaving] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  const reload = useCallback(() => {
    setRefreshKey((k) => k + 1)
  }, [])

  useEffect(() => {
    const load = async () => {
      const [schedRes, skillsRes] = await Promise.all([
        get<{ schedules: ReportSchedule[] }>('/api/ai/reports/schedules'),
        get<{ skills: SkillOption[] }>('/api/ai/skills'),
      ])
      if (schedRes) {
        setSchedules(schedRes.schedules)
      }
      if (skillsRes) {
        setSkills(skillsRes.skills)
      }
      setLoaded(true)
    }
    void load()
  }, [get, refreshKey])

  const skillName = (id: string) => skills.find((s) => s.id === id)?.name ?? id

  const handleSave = async () => {
    if (!form) {
      return
    }
    setFormError(null)
    setSaving(true)
    const payload = {
      name: form.name.trim(),
      skill_ids: form.skillIDs,
      recipients: form.recipients
        .split(',')
        .map((r) => r.trim())
        .filter(Boolean),
      cadence: form.cadence,
      hour_utc: Number(form.hourUTC),
      weekday: Number(form.weekday),
      day_of_month: Number(form.dayOfMonth),
      is_active: form.isActive,
    }
    const saved = form.id
      ? await putData<ReportSchedule>(`/api/ai/reports/schedules/${form.id}`, payload)
      : await postData<ReportSchedule>('/api/ai/reports/schedules', payload)
    setSaving(false)
    if (saved) {
      setForm(null)
      reload()
    } else {
      setFormError(t('admin.reports.save_failed'))
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) {
      return
    }
    await deleteData(`/api/ai/reports/schedules/${deleteTarget.id}`)
    setDeleteTarget(null)
    reload()
  }

  const cadenceLabel = (s: ReportSchedule) => {
    const hour = `${String(s.hour_utc).padStart(2, '0')}:00 UTC`
    if (s.cadence === 'weekly') {
      return `${t('admin.reports.cadence_weekly')} · ${t(`admin.reports.weekday_${s.weekday}` as Parameters<typeof t>[0])} ${hour}`
    }
    if (s.cadence === 'monthly') {
      return `${t('admin.reports.cadence_monthly')} · ${s.day_of_month}. ${hour}`
    }
    return `${t('admin.reports.cadence_daily')} · ${hour}`
  }

  if (!loaded && loading) {
    return <LoadingScreen />
  }

  return (
    <AdminPanelShell
      title={t('admin.reports.title')}
      description={t('admin.reports.description')}
      error={error}
      action={
        !form && (
          <Button onClick={() => setForm(emptyForm)}>{t('admin.reports.new_schedule')}</Button>
        )
      }
    >
      {form ? (
        <div className={legacyLayoutClass('page-stack')}>
          <FormField
            label={t('admin.reports.name')}
            value={form.name}
            onChange={(v) => setForm({ ...form, name: v })}
            required
          />
          <MultiSelect
            value={form.skillIDs}
            onChange={(v) => setForm({ ...form, skillIDs: v })}
            options={skills.map((s) => ({ value: s.id, label: s.name }))}
            header={t('admin.reports.skills')}
            ariaLabel={t('admin.reports.skills')}
          />
          <FormField
            label={t('admin.reports.recipients')}
            value={form.recipients}
            onChange={(v) => setForm({ ...form, recipients: v })}
            placeholder="a@example.com, b@example.com"
            required
          />
          <div className="flex flex-wrap gap-3">
            <Select
              value={form.cadence}
              onChange={(v) => setForm({ ...form, cadence: v })}
              options={[
                { value: 'daily', label: t('admin.reports.cadence_daily') },
                { value: 'weekly', label: t('admin.reports.cadence_weekly') },
                { value: 'monthly', label: t('admin.reports.cadence_monthly') },
              ]}
              ariaLabel={t('admin.reports.cadence')}
            />
            <Select
              value={form.hourUTC}
              onChange={(v) => setForm({ ...form, hourUTC: v })}
              options={Array.from({ length: 24 }, (_, h) => ({
                value: String(h),
                label: `${String(h).padStart(2, '0')}:00 UTC`,
              }))}
              ariaLabel={t('admin.reports.hour')}
            />
            {form.cadence === 'weekly' && (
              <Select
                value={form.weekday}
                onChange={(v) => setForm({ ...form, weekday: v })}
                options={Array.from({ length: 7 }, (_, d) => ({
                  value: String(d),
                  label: t(`admin.reports.weekday_${d}` as Parameters<typeof t>[0]),
                }))}
                ariaLabel={t('admin.reports.weekday')}
              />
            )}
            {form.cadence === 'monthly' && (
              <Select
                value={form.dayOfMonth}
                onChange={(v) => setForm({ ...form, dayOfMonth: v })}
                options={Array.from({ length: 28 }, (_, d) => ({
                  value: String(d + 1),
                  label: String(d + 1),
                }))}
                ariaLabel={t('admin.reports.day_of_month')}
              />
            )}
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form.isActive}
              onChange={(e) => setForm({ ...form, isActive: e.target.checked })}
            />
            {t('admin.reports.active')}
          </label>
          {formError && <p className="text-error m-0 text-sm">{formError}</p>}
          <div className="flex gap-2">
            <Button
              onClick={() => void handleSave()}
              disabled={saving || !form.name.trim() || form.skillIDs.length === 0}
            >
              {t('common.save')}
            </Button>
            <Button variant="secondary" onClick={() => setForm(null)}>
              {t('common.cancel')}
            </Button>
          </div>
        </div>
      ) : schedules.length === 0 ? (
        <EmptyState
          title={t('admin.reports.empty_title')}
          description={t('admin.reports.empty')}
          icon={<ReportScheduleEmptyIcon />}
          action={{ label: t('admin.reports.new_schedule'), onClick: () => setForm(emptyForm) }}
        />
      ) : (
        <div className={adminTableContainerClass}>
          <table className={adminTableClass}>
            <thead>
              <tr className={adminTheadRowClass}>
                <th className={adminThClass}>{t('admin.reports.name')}</th>
                <th className={adminThClass}>{t('admin.reports.cadence')}</th>
                <th className={adminThClass}>{t('admin.reports.skills')}</th>
                <th className={adminThClass}>{t('admin.reports.recipients')}</th>
                <th className={adminThClass}>{t('admin.reports.last_run')}</th>
                <th className={adminThClass}>{t('admin.reports.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {schedules.map((s) => (
                <tr key={s.id} className={adminTrClass}>
                  <td className={adminTdClass}>
                    {s.name}
                    {!s.is_active && (
                      <span className="text-foreground-muted ml-2 text-xs">
                        ({t('admin.reports.inactive')})
                      </span>
                    )}
                  </td>
                  <td className={adminTdClass}>{cadenceLabel(s)}</td>
                  <td className={adminTdClass}>{s.skill_ids.map(skillName).join(', ')}</td>
                  <td className={adminTdClass}>{s.recipients.join(', ')}</td>
                  <td className={adminTdClass}>
                    {s.last_run_at ? (
                      <>
                        {new Date(s.last_run_at).toLocaleString()}
                        <span
                          className={
                            s.last_status === 'success'
                              ? 'text-success ml-2 text-xs'
                              : 'text-error ml-2 text-xs'
                          }
                        >
                          {s.last_status}
                        </span>
                      </>
                    ) : (
                      t('admin.reports.never_run')
                    )}
                  </td>
                  <td className={adminTdClass}>
                    <div className="flex gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setForm(scheduleToForm(s))}
                      >
                        {t('common.edit')}
                      </Button>
                      <Button variant="danger" size="sm" onClick={() => setDeleteTarget(s)}>
                        {t('common.delete')}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <ConfirmDialog
        open={deleteTarget !== null}
        title={t('admin.reports.delete_confirm')}
        message={deleteTarget?.name}
        variant="danger"
        onConfirm={() => void handleDelete()}
        onCancel={() => setDeleteTarget(null)}
      />
    </AdminPanelShell>
  )
}

export { ReportSchedulesPanel }
export default ReportSchedulesPanel
