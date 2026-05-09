-- Revenue and order count per sales territory, with country/region breakdown.
-- Useful for the territory dimension in semantic models.
SELECT
  st.name              AS territory,
  st.countryregioncode AS country,
  st."group"           AS region_group,
  COUNT(soh.salesorderid) AS order_count,
  SUM(soh.totaldue)       AS total_revenue
FROM sales.salesorderheader soh
LEFT JOIN sales.salesterritory st ON soh.territoryid = st.territoryid
GROUP BY st.name, st.countryregioncode, st."group"
ORDER BY total_revenue DESC;
