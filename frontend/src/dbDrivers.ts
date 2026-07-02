import clickhouseLogo from './assets/db-logos/clickhouse.svg'
import databricksLogo from './assets/db-logos/databricks.svg'
import mysqlLogo from './assets/db-logos/mysql.png'
import oracleLogo from './assets/db-logos/oracle.svg'
import postgresLogo from './assets/db-logos/postgres.svg'
import snowflakeLogo from './assets/db-logos/snowflake.svg'
import sqliteLogo from './assets/db-logos/sqlite.svg'
import sqlserverLogo from './assets/db-logos/sqlserver.svg'
import type { TranslationKey } from './i18n'

export const DRIVER_IDS = [
  'postgres',
  'mysql',
  'sqlserver',
  'clickhouse',
  'sqlite',
  'snowflake',
  'databricks',
  'oracle',
] as const
export type DriverId = (typeof DRIVER_IDS)[number]

export const DRIVER_LOGOS: Record<DriverId, string> = {
  postgres: postgresLogo,
  mysql: mysqlLogo,
  sqlserver: sqlserverLogo,
  clickhouse: clickhouseLogo,
  sqlite: sqliteLogo,
  snowflake: snowflakeLogo,
  databricks: databricksLogo,
  oracle: oracleLogo,
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
    case 'sqlite':
    case 'sqlite3':
      return 'sqlite'
    case 'snowflake':
      return 'snowflake'
    case 'databricks':
    case 'spark':
    case 'dbx':
      return 'databricks'
    case 'oracle':
    case 'ora':
      return 'oracle'
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
    case 'databricks':
      return 443
    case 'oracle':
      return 1521
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
    case 'databricks':
      return { port: portStr, ssl_mode: '' }
    case 'oracle':
      return { port: portStr, ssl_mode: 'disable' }
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
      return '******host:5432/dbname?sslmode=disable'
    case 'mysql':
      return 'user:pass@tcp(host:3306)/dbname?parseTime=true&loc=UTC'
    case 'sqlserver':
      return '******host:1433?database=dbname&encrypt=disable'
    case 'clickhouse':
      return '******host:9000/default'
    case 'sqlite':
      return 'file:/path/to/database.db?mode=ro'
    case 'snowflake':
      return 'user:pass@account/dbname/schema?warehouse=WH'
    case 'databricks':
      return 'token:dapi***@host:443/sql/1.0/warehouses/abc?catalog=main'
    case 'oracle':
      return '******host:1521/service_name'
    default:
      return ''
  }
}

export interface DriverExtraField {
  key: string
  labelKey: TranslationKey
  required?: boolean
  placeholder?: string
}

export interface DriverFormSpec {
  host: boolean
  hostLabelKey?: TranslationKey
  port: boolean
  username: boolean
  password: boolean
  passwordLabelKey?: TranslationKey
  database: boolean
  databaseLabelKey?: TranslationKey
  databaseRequired?: boolean
  ssl: boolean
  extras: DriverExtraField[]
}

const FULL_FORM: DriverFormSpec = {
  host: true,
  port: true,
  username: true,
  password: true,
  database: true,
  ssl: true,
  extras: [],
}

const FORM_SPECS: Partial<Record<DriverId, DriverFormSpec>> = {
  sqlite: {
    host: false,
    port: false,
    username: false,
    password: false,
    database: true,
    databaseLabelKey: 'datasources.fields.file_path',
    databaseRequired: true,
    ssl: false,
    extras: [],
  },
  snowflake: {
    host: true,
    hostLabelKey: 'datasources.fields.account',
    port: false,
    username: true,
    password: true,
    database: true,
    databaseRequired: true,
    ssl: false,
    extras: [
      { key: 'warehouse', labelKey: 'datasources.fields.warehouse' },
      { key: 'role', labelKey: 'datasources.fields.role' },
      { key: 'schema', labelKey: 'datasources.fields.schema' },
    ],
  },
  databricks: {
    host: true,
    port: true,
    username: false,
    password: true,
    passwordLabelKey: 'datasources.fields.token',
    database: true,
    databaseLabelKey: 'datasources.fields.catalog',
    ssl: false,
    extras: [
      {
        key: 'http_path',
        labelKey: 'datasources.fields.http_path',
        required: true,
        placeholder: '/sql/1.0/warehouses/...',
      },
      { key: 'schema', labelKey: 'datasources.fields.schema' },
    ],
  },
  oracle: {
    host: true,
    port: true,
    username: true,
    password: true,
    database: true,
    databaseLabelKey: 'datasources.fields.service_name',
    databaseRequired: true,
    ssl: true,
    extras: [],
  },
}

export function driverFormSpec(id: string): DriverFormSpec {
  const d = normalizeDriverType(id)
  if ((DRIVER_IDS as readonly string[]).includes(d)) {
    return FORM_SPECS[d as DriverId] ?? FULL_FORM
  }
  return FULL_FORM
}
