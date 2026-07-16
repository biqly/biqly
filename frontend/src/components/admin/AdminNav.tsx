import { useT } from '../../i18n'
import {
  adminCategoryTabsClass,
  adminNavClass,
  adminPanelPillClass,
  adminPanelPillListClass,
  adminTabButtonClass,
} from './adminClasses'
import {
  ADMIN_NAV_GROUPS,
  ADMIN_TAB_LABEL_KEYS,
  adminGroupForTab,
  type AdminTab,
} from './adminNavConfig'

interface AdminNavProps {
  activeTab: AdminTab
  onTabChange: (tab: AdminTab) => void
  onTabHover?: (tab: AdminTab) => void
}

export function AdminNav({ activeTab, onTabChange, onTabHover }: AdminNavProps) {
  const t = useT()
  const activeGroup = adminGroupForTab(activeTab)

  return (
    <nav className={adminNavClass} aria-label={t('admin.nav.label')}>
      <div className={adminCategoryTabsClass} role="tablist" aria-label={t('admin.nav.label')}>
        {ADMIN_NAV_GROUPS.map((group) => {
          const isActive = group.id === activeGroup.id
          const firstTab = group.tabs[0]!
          return (
            <button
              key={group.id}
              type="button"
              role="tab"
              id={`admin-category-tab-${group.id}`}
              aria-selected={isActive}
              className={adminTabButtonClass(isActive)}
              onClick={() => {
                if (!isActive) {
                  onTabChange(firstTab)
                }
              }}
              onMouseEnter={() => onTabHover?.(firstTab)}
              onFocus={() => onTabHover?.(firstTab)}
            >
              {t(group.labelKey)}
            </button>
          )
        })}
      </div>

      <div className={adminPanelPillListClass} role="tablist" aria-label={t(activeGroup.labelKey)}>
        {activeGroup.tabs.map((tab) => {
          const isActive = tab === activeTab
          return (
            <button
              key={tab}
              type="button"
              role="tab"
              id={`admin-tab-${tab}`}
              aria-selected={isActive}
              aria-controls="admin-active-tabpanel"
              className={adminPanelPillClass(isActive)}
              onClick={() => onTabChange(tab)}
              onMouseEnter={() => onTabHover?.(tab)}
              onFocus={() => onTabHover?.(tab)}
            >
              {t(ADMIN_TAB_LABEL_KEYS[tab])}
            </button>
          )
        })}
      </div>
    </nav>
  )
}
