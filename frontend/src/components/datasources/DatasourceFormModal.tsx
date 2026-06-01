import {
  DRIVER_IDS,
  driverDefaultPort,
  driverDsnPlaceholder,
  driverStructuredDefaults,
  isInsecureSslMode,
} from '../../dbDrivers'
import { useT } from '../../i18n'
import { DriverTileGrid } from '../DriverTileGrid'
import { Modal } from '../ui/Modal'

type ConnectionMode = 'structured' | 'raw'

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

export type DatasourceFormModalProps = {
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
  const driverConnHints = driverStructuredDefaults(form.type)
  const defaultPortHint = driverDefaultPort(form.type)

  return (
    <Modal
      open={open}
      title={editingId ? t('datasources.edit_title') : t('datasources.new')}
      subtitle={t('datasources.form_subtitle')}
      onClose={onClose}
      className="modal-card--datasource"
    >
      <div className="form-stack">
        <div className="form-group">
          <span className="form-group__label">{t('datasources.connection_mode')}</span>
          <div className="conn-mode-segmented" role="group" aria-label={t('datasources.connection_mode')}>
            <button
              type="button"
              className={`conn-mode-segmented__btn${connMode === 'structured' ? ' conn-mode-segmented__btn--active' : ''}`}
              aria-pressed={connMode === 'structured'}
              onClick={() => onConnModeChange('structured')}
            >
              <span className="conn-mode-segmented__title">{t('datasources.mode_structured')}</span>
              <span className="conn-mode-segmented__desc">{t('datasources.mode_structured_desc')}</span>
            </button>
            <button
              type="button"
              className={`conn-mode-segmented__btn${connMode === 'raw' ? ' conn-mode-segmented__btn--active' : ''}`}
              aria-pressed={connMode === 'raw'}
              onClick={() => onConnModeChange('raw')}
            >
              <span className="conn-mode-segmented__title">{t('datasources.mode_raw')}</span>
              <span className="conn-mode-segmented__desc">{t('datasources.mode_raw_desc')}</span>
            </button>
          </div>
        </div>

        <div className="form-group">
          <label htmlFor="datasource-name">{t('datasources.name')}</label>
          <input
            id="datasource-name"
            name="name"
            value={form.name}
            onChange={(e) => onFormChange({ ...form, name: e.target.value })}
            placeholder="prod-orders-db"
            autoComplete="off"
            autoFocus
          />
        </div>

        <div className="form-group">
          <span className="form-group__label">{t('datasources.type')}</span>
          <DriverTileGrid
            value={form.type}
            onChange={onDriverChange}
            ids={DRIVER_IDS}
            ariaLabel={t('datasources.pick_driver')}
            t={t}
          />
        </div>

        {connMode === 'raw' ? (
          <div className="form-group">
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
            <small className="form-hint">
              {editingId ? t('datasources.dsn_keep_hint') : t('datasources.dsn_hint')}
            </small>
          </div>
        ) : (
          <>
            <div className="modal-form-row">
              <div className="form-group">
                <label htmlFor="ds-host">{t('datasources.fields.host')}</label>
                <input
                  id="ds-host"
                  value={structured.host}
                  onChange={(e) => onStructuredChange({ ...structured, host: e.target.value })}
                  placeholder="localhost"
                  autoComplete="off"
                />
              </div>
              <div className="form-group">
                <label htmlFor="ds-port">{t('datasources.fields.port')}</label>
                <input
                  id="ds-port"
                  value={structured.port}
                  onChange={(e) => onStructuredChange({ ...structured, port: e.target.value })}
                  placeholder={defaultPortHint > 0 ? String(defaultPortHint) : ''}
                  inputMode="numeric"
                  autoComplete="off"
                />
                <small className="form-hint">{t('common.optional')}</small>
              </div>
            </div>
            <div className="modal-form-row">
              <div className="form-group">
                <label htmlFor="ds-db">{t('datasources.fields.database')}</label>
                <input
                  id="ds-db"
                  value={structured.database_name}
                  onChange={(e) => onStructuredChange({ ...structured, database_name: e.target.value })}
                  autoComplete="off"
                />
              </div>
              <div className="form-group">
                <label htmlFor="ds-user">{t('datasources.fields.username')}</label>
                <input
                  id="ds-user"
                  value={structured.username}
                  onChange={(e) => onStructuredChange({ ...structured, username: e.target.value })}
                  autoComplete="off"
                />
              </div>
            </div>
            <div className="form-group">
              <label htmlFor="ds-pass">{t('datasources.fields.password')}</label>
              <input
                id="ds-pass"
                type="password"
                value={structured.password}
                onChange={(e) => onStructuredChange({ ...structured, password: e.target.value })}
                autoComplete="off"
              />
              <small className="form-hint">
                {editingId ? t('datasources.password_keep_hint') : t('datasources.dsn_hint')}
              </small>
            </div>
            <div className="form-group">
              <label htmlFor="ds-ssl">{t('datasources.fields.ssl_mode')}</label>
              <input
                id="ds-ssl"
                value={structured.ssl_mode}
                onChange={(e) => onStructuredChange({ ...structured, ssl_mode: e.target.value })}
                placeholder={driverConnHints.ssl_mode || 'disable'}
                autoComplete="off"
              />
              <small className="form-hint">{t('common.optional')}</small>
              {isInsecureSslMode(structured.ssl_mode) && (
                <small className="form-hint form-hint--warning">
                  ⚠ {t('datasources.ssl_insecure_warning')}
                </small>
              )}
            </div>
          </>
        )}

        {draftTestResult && (
          <p className="datasource-form-test-result" role="status">
            {draftTestResult}
          </p>
        )}
      </div>

      <div className="modal-actions datasource-form-actions">
        <button className="btn" type="button" onClick={onClose} disabled={loading}>
          {t('datasources.cancel')}
        </button>
        <button className="btn" type="button" onClick={onTest} disabled={loading || !canSubmit}>
          {t('datasources.test_before_save')}
        </button>
        <button className="btn btn-primary" type="button" onClick={onSave} disabled={loading || !canSubmit}>
          {editingId ? t('datasources.save') : t('datasources.create')}
        </button>
      </div>
    </Modal>
  )
}
