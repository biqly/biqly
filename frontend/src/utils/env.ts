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

function optionalString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function resolveAdminApiKey(): string {
  const runtime = typeof window !== 'undefined' ? window.__BIQLY_ENV__?.adminApiKey : ''
  if (runtime) {
    return runtime
  }
  return optionalString(import.meta.env.VITE_BI_ADMIN_API_KEY)
}

export const frontendEnv: FrontendEnv = {
  adminApiKey: resolveAdminApiKey(),
}
