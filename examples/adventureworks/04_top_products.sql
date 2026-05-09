-- Top 20 products by revenue and units sold.
-- Joins order detail to the product catalog and aggregates linetotal.
SELECT
  p.productid,
  p.name                  AS product,
  p.productnumber,
  SUM(sod.orderqty)       AS units_sold,
  SUM(sod.linetotal)      AS revenue
FROM sales.salesorderdetail sod
JOIN production.product p ON sod.productid = p.productid
GROUP BY p.productid, p.name, p.productnumber
ORDER BY revenue DESC
LIMIT 20;
