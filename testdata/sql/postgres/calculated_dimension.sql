SELECT
  COALESCE(orders.first_name, '') || ' ' || COALESCE(orders.last_name, '') AS "full_name",
  COUNT("orders"."id") AS "order_count"
FROM
  "public"."orders"
GROUP BY
  COALESCE(orders.first_name, '') || ' ' || COALESCE(orders.last_name, '')
ORDER BY
  "order_count" DESC
LIMIT 50
