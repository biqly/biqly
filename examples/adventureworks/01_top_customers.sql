-- Top 10 customers by lifetime revenue.
-- Joins sales.salesorderheader to sales.customer and aggregates totaldue.
SELECT
  c.customerid,
  c.accountnumber,
  COUNT(soh.salesorderid) AS order_count,
  SUM(soh.totaldue)       AS total_revenue
FROM sales.salesorderheader soh
JOIN sales.customer c ON soh.customerid = c.customerid
GROUP BY c.customerid, c.accountnumber
ORDER BY total_revenue DESC
LIMIT 10;
