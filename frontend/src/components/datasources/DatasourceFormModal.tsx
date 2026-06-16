import {
  DRIVER_IDS,
  driverDefaultPort,
  driverDsnPlaceholder,
  driverStructuredDefaults,
  isInsecureSslMode,
} from '../../dbDrivers'
import { useAutofocus } from '../../hooks/useAutofocus'
import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { formStackClass, legacyFormClass } from '../../lib/formClasses'
import { modalActionsBorderedClass, modalFormRowClass } from '../../lib/modalClasses'
import { DriverTileGrid } from '../DriverTileGrid'
import { Modal } from '../ui/Modal'

type ConnectionMode = 'structured' | 'raw'

const CONN_MODE_BTN_BASE =
  'm-0 flex cursor-pointer flex-col items-start gap-[0.15rem] rounded-[0.4rem] border bg-transparent px-[0.65rem] py-[0.55rem] text-left text-caption leading-tight font-medium transition-all duration-150 ease-out'
const CONN_MODE_BTN_ACTIVE =
  'bg-card text-foreground border-[color-mix(in_srgb,var(--accent)_45%,var(--border))] shadow-[0_1px_2px_rgba(0,0,0,0.12)]'
const CONN_MODE_BTN_INACTIVE =
  'text-foreground-muted hover:text-foreground border-transparent hover:bg-[color-mix(in_srgb,var(--bg-card)_70%,transparent)]'
const CONN_MODE_DESC_ACTIVE = 'text-foreground-muted'
const CONN_MODE_DESC_INACTIVE = 'text-foreground-faint'

interface StructuredForm {
  host: string
  port: string
  username: string
  password: string
  database_name: string
  ssl_mode: string
}

interface DatasourceForm {
  name: string
  type: string
  dsn: string
}

export interface DatasourceFormModalProps {
  open: boolean
  editingId: string | null
  connMode: ConnectionMode
  form: DatasourceForm
  structured: StructuredForm
  draftTestResult: string | null
  loading: boolean
  canSubmit: boolean
  onClose: () => void
  onConnModeChange: (mode: ConnectionMode) => void
  onFormChange: (form: DatasourceForm) => void
  onStructuredChange: (structured: StructuredForm) => void
  onDriverChange: (type: string) => void
  onTest: () => void
  onSave: () => void
}

export function DatasourceFormModal({
  open,
  editingId,
  connMode,
  form,
  structured,
  draftTestResult,
  loading,
  canSubmit,
  onClose,
  onConnModeChange,
  onFormChange,
  onStructuredChange,
  onDriverChange,
  onTest,
  onSave,
}: DatasourceFormModalProps) {
  const t = useT()
  const datasourceNameInputRef = useAutofocus<HTMLInputElement>(open)
  const driverConnHints = driverStructuredDefaults(form.type)
  const defaultPortHint = driverDefaultPort(form.type)

  return (
    <Modal
      open={open}
      title={editingId ? t('datasources.edit_title') : t('datasources.new')}
      subtitle={t('datasources.form_subtitle')}
      onClose={onClose}
      className="w-full sm:max-w-136"
      bodyClassName="gap-0"
    >
      <div className={formStackClass}>
        <div className={legacyFormClass('form-group')}>
          <span className="mb-[0.35rem] block text-[0.875rem] font-semibold">
            {t('datasources.connection_mode')}
          </span>
          <div
            className={legacyCardClass(
              'border-border bg-card-raised grid w-full grid-cols-2 gap-1 rounded-lg border p-1',
            )}
            role="group"
            aria-label={t('datasources.connection_mode')}
          >
            <button
              type="button"
              className={cn(
                CONN_MODE_BTN_BASE,
                connMode === 'structured' ? CONN_MODE_BTN_ACTIVE : CONN_MODE_BTN_INACTIVE,
              )}
              aria-pressed={connMode === 'structured'}
              onClick={() => onConnModeChange('structured')}
            >
              <span className="text-caption font-semibold">{t('datasources.mode_structured')}</span>
              <span
                className={cn(
                  'text-[0.7rem] leading-[1.3] font-normal',
                  connMode === 'structured' ? CONN_MODE_DESC_ACTIVE : CONN_MODE_DESC_INACTIVE,
                )}
              >
                {t('datasources.mode_structured_desc')}
              </span>
            </button>
            <button
              type="button"
              className={cn(
                CONN_MODE_BTN_BASE,
                connMode === 'raw' ? CONN_MODE_BTN_ACTIVE : CONN_MODE_BTN_INACTIVE,
              )}
              aria-pressed={connMode === 'raw'}
              onClick={() => onConnModeChange('raw')}
            >
              <span className="text-caption font-semibold">{t('datasources.mode_raw')}</span>
              <span
                className={cn(
                  'text-[0.7rem] leading-[1.3] font-normal',
                  connMode === 'raw' ? CONN_MODE_DESC_ACTIVE : CONN_MODE_DESC_INACTIVE,
                )}
              >
                {t('datasources.mode_raw_desc')}
              </span>
            </button>
          </div>
        </div>

        <div className={legacyFormClass('form-group')}>
          <label htmlFor="datasource-name">{t('datasources.name')}</label>
          <input
            id="datasource-name"
            name="name"
            ref={datasourceNameInputRef}
            value={form.name}
            onChange={(e) => onFormChange({ ...form, name: e.target.value })}
            placeholder="prod-orders-db"
            autoComplete="off"
          />
        </div>

        <div className={legacyFormClass('form-group')}>
          <span className="mb-[0.35rem] block text-[0.875rem] font-semibold">
            {t('datasources.type')}
          </span>
          <DriverTileGrid
            value={form.type}
            onChange={onDriverChange}
            ids={DRIVER_IDS}
            ariaLabel={t('datasources.pick_driver')}
            t={t}
          />
        </div>

        {connMode === 'raw' ? (
          <div className={legacyFormClass('form-group')}>
            <label htmlFor="datasource-dsn">{t('datasources.dsn')}</label>
            <input
              id="datasource-dsn"
              name="dsn"
              type="password"
              value={form.dsn}
              onChange={(e) => onFormChange({ ...form, dsn: e.target.value })}
              placeholder={driverDsnPlaceholder(form.type)}
              autoComplete="off"
              spellCheck={false}
            />
            <small className="text-foreground-muted mt-1 block text-[0.75rem] leading-[1.35]">
              {editingId ? t('datasources.dsn_keep_hint') : t('datasources.dsn_hint')}
            </small>
          </div>
        ) : (
          <>
            <div className={modalFormRowClass()}>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="ds-host">{t('datasources.fields.host')}</label>
                <input
                  id="ds-host"
                  value={structured.host}
                  onChange={(e) => onStructuredChange({ ...structured, host: e.target.value })}
                  placeholder="localhost"
                  autoComplete="off"
                />
              </div>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="ds-port">{t('datasources.fields.port')}</label>
                <input
                  id="ds-port"
                  value={structured.port}
                  onChange={(e) => onStructuredChange({ ...structured, port: e.target.value })}
                  placeholder={defaultPortHint > 0 ? String(defaultPortHint) : ''}
                  inputMode="numeric"
                  autoComplete="off"
                />
                <small className="text-foreground-muted mt-1 block text-[0.75rem] leading-[1.35]">
                  {t('common.optional')}
                </small>
              </div>
            </div>
            <div className={modalFormRowClass()}>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="ds-db">{t('datasources.fields.database')}</label>
                <input
                  id="ds-db"
                  value={structured.database_name}
                  onChange={(e) =>
                    onStructuredChange({ ...structured, database_name: e.target.value })
                  }
                  autoComplete="off"
                />
              </div>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="ds-user">{t('datasources.fields.username')}</label>
                <input
                  id="ds-user"
                  value={structured.username}
                  onChange={(e) => onStructuredChange({ ...structured, username: e.target.value })}
                  autoComplete="off"
                />
              </div>
            </div>
            <div className={legacyFormClass('form-group')}>
              <label htmlFor="ds-pass">{t('datasources.fields.password')}</label>
              <input
                id="ds-pass"
                type="password"
                value={structured.password}
                onChange={(e) => onStructuredChange({ ...structured, password: e.target.value })}
                autoComplete="off"
              />
              <small className="text-foreground-muted mt-1 block text-[0.75rem] leading-[1.35]">
                {editingId ? t('datasources.password_keep_hint') : t('datasources.dsn_hint')}
              </small>
            </div>
            <div className={legacyFormClass('form-group')}>
              <label htmlFor="ds-ssl">{t('datasources.fields.ssl_mode')}</label>
              <input
                id="ds-ssl"
                value={structured.ssl_mode}
                onChange={(e) => onStructuredChange({ ...structured, ssl_mode: e.target.value })}
                placeholder={driverConnHints.ssl_mode || 'disable'}
                autoComplete="off"
              />
              <small className="text-foreground-muted mt-1 block text-[0.75rem] leading-[1.35]">
                {t('common.optional')}
              </small>
              {isInsecureSslMode(structured.ssl_mode) && (
                <small className="text-warning mt-1 block text-[0.75rem] leading-[1.35]">
                  ⚠ {t('datasources.ssl_insecure_warning')}
                </small>
              )}
            </div>
          </>
        )}

        {draftTestResult && (
          <p
            className={cn(
              'm-0 rounded-[0.45rem] border px-[0.65rem] py-[0.55rem] text-[0.8rem] leading-[1.4]',
              draftTestResult.includes('successful') || draftTestResult.includes('başarılı')
                ? 'text-success border-[color-mix(in_srgb,var(--success)_30%,var(--border))] bg-[color-mix(in_srgb,var(--success)_10%,transparent)]'
                : 'border-border bg-card-raised text-foreground-muted',
            )}
            role="status"
          >
            {draftTestResult}
          </p>
        )}
      </div>

      <div className={modalActionsBorderedClass()}>
        <button
          className={legacyButtonClass('btn')}
          type="button"
          onClick={onClose}
          disabled={loading}
        >
          {t('datasources.cancel')}
        </button>
        <button
          className={legacyButtonClass('btn')}
          type="button"
          onClick={onTest}
          disabled={loading || !canSubmit}
        >
          {t('datasources.test_before_save')}
        </button>
        <button
          className={legacyButtonClass('btn btn-primary')}
          type="button"
          onClick={onSave}
          disabled={loading || !canSubmit}
        >
          {editingId ? t('datasources.save') : t('datasources.create')}
        </button>
      </div>
    </Modal>
  )
}
