SELECT
  "customers"."region" AS "customer_region",
  SUM("orders"."total_amount") AS "total_revenue"
FROM
  "public"."orders"
LEFT JOIN "public"."customers" ON "public"."orders"."customer_id" = "public"."customers"."id"
GROUP BY
  "customers"."region"
ORDER BY
  "total_revenue" DESC
LIMIT 50
