import { useT } from '../../i18n'
import {
  adminNavClass,
  adminNavDesktopClass,
  adminNavGroupClass,
  adminNavGroupTitleClass,
  adminNavLinkClass,
  adminNavListClass,
  adminNavMobileClass,
  adminNavMobileLabelClass,
  adminNavSelectClass,
} from './adminClasses'
import { ADMIN_NAV_GROUPS, ADMIN_TAB_LABEL_KEYS, type AdminTab } from './adminNavConfig'

interface AdminNavProps {
  activeTab: AdminTab
  onTabChange: (tab: AdminTab) => void
  onTabHover?: (tab: AdminTab) => void
}

export function AdminNav({ activeTab, onTabChange, onTabHover }: AdminNavProps) {
  const t = useT()

  return (
    <nav className={adminNavClass} aria-label={t('admin.nav.label')}>
      <div className={adminNavMobileClass}>
        <label className={adminNavMobileLabelClass} htmlFor="admin-nav-select">
          {t('admin.nav.jump_to')}
        </label>
        <select
          id="admin-nav-select"
          className={adminNavSelectClass}
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

      <div className={adminNavDesktopClass}>
        {ADMIN_NAV_GROUPS.map((group) => (
          <div
            key={group.id}
            className={adminNavGroupClass}
            role="group"
            aria-label={t(group.labelKey)}
          >
            <p className={adminNavGroupTitleClass} id={`admin-nav-group-${group.id}`}>
              {t(group.labelKey)}
            </p>
            <ul className={adminNavListClass} aria-labelledby={`admin-nav-group-${group.id}`}>
              {group.tabs.map((tab) => {
                const isActive = activeTab === tab
                return (
                  <li key={tab}>
                    <button
                      type="button"
                      className={adminNavLinkClass(isActive)}
                      onClick={() => onTabChange(tab)}
                      onMouseEnter={() => onTabHover?.(tab)}
                      onFocus={() => onTabHover?.(tab)}
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
