import clickhouseLogo from './assets/db-logos/clickhouse.svg'
import mysqlLogo from './assets/db-logos/mysql.png'
import postgresLogo from './assets/db-logos/postgres.svg'
import sqlserverLogo from './assets/db-logos/sqlserver.svg'
import type { TranslationKey } from './i18n'

export const DRIVER_IDS = ['postgres', 'mysql', 'sqlserver', 'clickhouse'] as const
export type DriverId = (typeof DRIVER_IDS)[number]

export const DRIVER_LOGOS: Record<DriverId, string> = {
  postgres: postgresLogo,
  mysql: mysqlLogo,
  sqlserver: sqlserverLogo,
  clickhouse: clickhouseLogo,
}

export function driverLogoUrl(id: string): string | undefined {
  if ((DRIVER_IDS as readonly string[]).includes(id)) {
    return DRIVER_LOGOS[id as DriverId]
  }
  return undefined
}

export function driverLabelKey(id: string): TranslationKey {
  if ((DRIVER_IDS as readonly string[]).includes(id)) {
    return `datasources.drivers.${id}` as TranslationKey
  }
  return 'datasources.drivers.unknown'
}

function normalizeDriverType(t: string): string {
  switch (t.trim().toLowerCase()) {
    case 'postgresql':
    case 'postgres':
    case 'pg':
      return 'postgres'
    case 'mysql':
    case 'mariadb':
      return 'mysql'
    case 'sqlserver':
    case 'mssql':
      return 'sqlserver'
    case 'clickhouse':
    case 'ch':
      return 'clickhouse'
    default:
      return t.trim().toLowerCase()
  }
}

export function driverDefaultPort(id: string): number {
  switch (normalizeDriverType(id)) {
    case 'postgres':
      return 5432
    case 'mysql':
      return 3306
    case 'sqlserver':
      return 1433
    case 'clickhouse':
      return 9000
    default:
      return 0
  }
}

export function driverStructuredDefaults(id: string): { port: string; ssl_mode: string } {
  const d = normalizeDriverType(id)
  const port = driverDefaultPort(id)
  const portStr = port > 0 ? String(port) : ''
  switch (d) {
    case 'postgres':
      return { port: portStr, ssl_mode: 'require' }
    case 'mysql':
      return { port: portStr, ssl_mode: 'true' }
    case 'sqlserver':
      return { port: portStr, ssl_mode: 'require' }
    case 'clickhouse':
      return { port: portStr, ssl_mode: 'true' }
    default:
      return { port: '', ssl_mode: '' }
  }
}

const SSL_INSECURE_VALUES = new Set(['disable', 'disabled', 'off', 'false', '0', 'no'])

export function isInsecureSslMode(value: string | null | undefined): boolean {
  if (!value) {
    return false
  }
  return SSL_INSECURE_VALUES.has(value.trim().toLowerCase())
}

export function driverDsnPlaceholder(id: string): string {
  switch (normalizeDriverType(id)) {
    case 'postgres':
      return 'postgres://user:pass@host:5432/dbname?sslmode=disable'
    case 'mysql':
      return 'user:pass@tcp(host:3306)/dbname?parseTime=true&loc=UTC'
    case 'sqlserver':
      return 'sqlserver://user:pass@host:1433?database=dbname&encrypt=disable'
    case 'clickhouse':
      return 'clickhouse://user:pass@host:9000/default'
    default:
      return ''
  }
}
