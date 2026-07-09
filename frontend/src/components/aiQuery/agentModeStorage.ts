const AGENT_MODE_STORAGE_KEY = 'biqly.aiQuery.agentMode'

// Agent mode defaults OFF (unlike the auto-find-skills toggle, which defaults
// on): it's a new, experimental send path a user opts into, not a preference
// that changes existing default behavior. Only an explicit "true" turns it
// on. Guarded so a storage-less environment (SSR/tests) simply falls back to
// the default. Split into its own module (rather than living in AIQuery.tsx
// alongside its sibling loadAutoFindEnabled/saveAutoFindEnabled) so it can be
// exported for the toggle-persistence unit test without tripping
// react-refresh/only-export-components on a component file.
export function loadAgentModeEnabled(): boolean {
  try {
    return window.localStorage.getItem(AGENT_MODE_STORAGE_KEY) === 'true'
  } catch {
    return false
  }
}

export function saveAgentModeEnabled(enabled: boolean): void {
  try {
    window.localStorage.setItem(AGENT_MODE_STORAGE_KEY, enabled ? 'true' : 'false')
  } catch {
    // Non-fatal: the toggle still works for the session without persistence.
  }
}
