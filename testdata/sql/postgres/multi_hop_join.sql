-- Golden test: multi-hop join with mixed relationship types
-- Scenario: orders -> order_items -> products (many_to_one + one_to_many)
-- Expected: planner detects mixed relationships, validates path
SELECT
  "customers"."name" AS "customer_name",
  "products"."name" AS "product_name",
  COUNT("order_items"."id") AS "item_count"
FROM
  "public"."orders"
LEFT JOIN "public"."customers" ON "public"."orders"."customer_id" = "public"."customers"."id"
LEFT JOIN "public"."order_items" ON "public"."orders"."id" = "public"."order_items"."order_id"
LEFT JOIN "public"."products" ON "public"."order_items"."product_id" = "public"."products"."id"
WHERE
  "orders"."status" = $1
GROUP BY
  "customers"."name",
  "products"."name"
ORDER BY
  "item_count" DESC
LIMIT 100
