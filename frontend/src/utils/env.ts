interface FrontendEnv {
  adminApiKey: string
}

declare global {
  interface Window {
    __BIQLY_ENV__?: {
      adminApiKey?: string
    }
  }
}

export function resolveAdminApiKey(): string {
  return typeof window !== 'undefined' ? (window.__BIQLY_ENV__?.adminApiKey ?? '') : ''
}

export const frontendEnv: FrontendEnv = {
  get adminApiKey() {
    return resolveAdminApiKey()
  },
}
