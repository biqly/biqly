SELECT
  "customers"."country" AS "country",
  COUNT("orders"."id") AS "order_count"
FROM
  "public"."orders"
LEFT JOIN "public"."customers" ON "public"."orders"."customer_id" = "public"."customers"."id"
WHERE
  "orders"."created_at" >= $1
GROUP BY
  "customers"."country"
ORDER BY
  "order_count" DESC
LIMIT 100
