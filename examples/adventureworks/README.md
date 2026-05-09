# AdventureWorks Sample Queries

Demo BI queries that run against the AdventureWorks dataset seeded by
`docker compose up` (see `scripts/init-adventureworks.sh`).

## Setup

```bash
make seed-adventureworks   # one-time clone into testdata/adventureworks/
docker compose up -d
```

Connect:

```bash
psql -h localhost -p 5433 -U test_user -d Adventureworks
# password: test_password
```

The DB name is mixed-case (`Adventureworks`) — quote it in shells that fold
identifiers, but the URL/CLI form above is fine as-is.

## Queries

| File | What it shows |
| --- | --- |
| `01_top_customers.sql` | Top 10 customers by lifetime revenue |
| `02_monthly_sales.sql` | Monthly order count, revenue, avg order value |
| `03_sales_by_territory.sql` | Revenue per sales territory and country/region |
| `04_top_products.sql` | Top 20 products by revenue and units sold |

Run any of them directly:

```bash
psql -h localhost -p 5433 -U test_user -d Adventureworks \
  -f examples/adventureworks/02_monthly_sales.sql
```

## Schema notes

AdventureWorks-for-Postgres preserves the original SQL Server CamelCase names
in `CREATE TABLE` statements unquoted, so PostgreSQL folds them to lowercase.
That means tables and columns are referenced as `sales.salesorderheader`,
`soh.totaldue`, etc. Schema names: `person`, `humanresources`, `production`,
`purchasing`, `sales`. The reserved word `group` (a column on
`sales.salesterritory`) must be double-quoted.
