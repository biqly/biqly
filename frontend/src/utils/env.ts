interface FrontendEnv {
  adminApiKey: string
}

function optionalString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

export const frontendEnv: FrontendEnv = {
  adminApiKey: optionalString(import.meta.env.VITE_BI_ADMIN_API_KEY),
}
