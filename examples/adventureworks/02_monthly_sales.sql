-- Monthly sales totals across the entire AdventureWorks order history.
-- Buckets orderdate by month and reports order count + revenue.
SELECT
  date_trunc('month', soh.orderdate)::date AS month,
  COUNT(soh.salesorderid)                  AS order_count,
  SUM(soh.totaldue)                        AS total_revenue,
  AVG(soh.totaldue)::numeric(12, 2)        AS avg_order_value
FROM sales.salesorderheader soh
GROUP BY date_trunc('month', soh.orderdate)
ORDER BY month;
