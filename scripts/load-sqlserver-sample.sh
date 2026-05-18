#!/usr/bin/env bash
set -euo pipefail

SQL_HOST="${SQL_HOST:-test-sqlserver}"
SQL_USER="${SQL_USER:-sa}"
SQL_PASSWORD="${MSSQL_SA_PASSWORD:?MSSQL_SA_PASSWORD is required}"
SQLCMD="${SQLCMD:-/opt/mssql-tools/bin/sqlcmd}"
SAMPLE_DIR="${SAMPLE_DIR:-/sample}"
INIT_SQL="${INIT_SQL:-/init.sql}"

CREATE_SQL="${SAMPLE_DIR}/BikeStores Sample Database - create objects.sql"
LOAD_SQL="${SAMPLE_DIR}/BikeStores Sample Database - load data.sql"

sqlcmd_base() {
  "${SQLCMD}" -S "${SQL_HOST}" -U "${SQL_USER}" -P "${SQL_PASSWORD}" -C "$@"
}

echo "==> SQL Server base init (${INIT_SQL})"
sqlcmd_base -i "${INIT_SQL}"

sample_loaded() {
  local result
  result="$(sqlcmd_base -Q "
SET NOCOUNT ON;
IF DB_ID(N'BikeStores') IS NOT NULL
   AND EXISTS (
     SELECT 1
     FROM BikeStores.INFORMATION_SCHEMA.TABLES
     WHERE TABLE_SCHEMA = N'sales' AND TABLE_NAME = N'orders'
   )
   AND (SELECT COUNT_BIG(*) FROM BikeStores.sales.orders) > 0
  SELECT 1
ELSE
  SELECT 0;
" -h -1 -W 2>/dev/null | tr -d '[:space:]')"
  [[ "${result}" == "1" ]]
}

if sample_loaded; then
  echo "==> BikeStores sample already loaded — skipping"
else
  echo "==> Creating BikeStores database"
  sqlcmd_base -Q "IF DB_ID(N'BikeStores') IS NULL CREATE DATABASE BikeStores;"

  echo "==> BikeStores schema and tables"
  sqlcmd_base -d BikeStores -i "${CREATE_SQL}"

  echo "==> BikeStores data (this may take a minute)"
  sqlcmd_base -d BikeStores -i "${LOAD_SQL}"

  echo "==> BikeStores sample loaded"
fi

echo "==> Grant test_user access to BikeStores"
sqlcmd_base -Q "
IF DB_ID(N'BikeStores') IS NOT NULL
BEGIN
  USE BikeStores;
  IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = N'test_user')
  BEGIN
    CREATE USER test_user FOR LOGIN test_user;
    ALTER ROLE db_owner ADD MEMBER test_user;
  END
END
"

echo "==> SQL Server init complete"
