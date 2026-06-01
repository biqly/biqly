import { useT } from '../../i18n'
import {
  ADMIN_NAV_GROUPS,
  ADMIN_TAB_LABEL_KEYS,
  type AdminTab,
} from './adminNavConfig'

type AdminNavProps = {
  activeTab: AdminTab
  onTabChange: (tab: AdminTab) => void
}

export function AdminNav({ activeTab, onTabChange }: AdminNavProps) {
  const t = useT()

  return (
    <nav className="admin-nav" aria-label={t('admin.nav.label')}>
      <div className="admin-nav__mobile">
        <label className="admin-nav__mobile-label" htmlFor="admin-nav-select">
          {t('admin.nav.jump_to')}
        </label>
        <select
          id="admin-nav-select"
          className="admin-nav__select"
          value={activeTab}
          onChange={(e) => onTabChange(e.target.value as AdminTab)}
        >
          {ADMIN_NAV_GROUPS.map((group) => (
            <optgroup key={group.id} label={t(group.labelKey)}>
              {group.tabs.map((tab) => (
                <option key={tab} value={tab}>
                  {t(ADMIN_TAB_LABEL_KEYS[tab])}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </div>

      <div className="admin-nav__desktop">
        {ADMIN_NAV_GROUPS.map((group) => (
          <div key={group.id} className="admin-nav__group" role="group" aria-label={t(group.labelKey)}>
            <p className="admin-nav__group-title" id={`admin-nav-group-${group.id}`}>
              {t(group.labelKey)}
            </p>
            <ul className="admin-nav__list" aria-labelledby={`admin-nav-group-${group.id}`}>
              {group.tabs.map((tab) => {
                const isActive = activeTab === tab
                return (
                  <li key={tab}>
                    <button
                      type="button"
                      className={`admin-nav__link${isActive ? ' is-active' : ''}`}
                      onClick={() => onTabChange(tab)}
                      aria-current={isActive ? 'page' : undefined}
                    >
                      {t(ADMIN_TAB_LABEL_KEYS[tab])}
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </div>
    </nav>
  )
}
