SELECT
  [users].[name] AS [name],
  [users].[email] AS [email]
FROM
  [public].[users]
WHERE
  [users].[age] >= @p1
ORDER BY (SELECT NULL) OFFSET 0 ROWS FETCH NEXT 50 ROWS ONLY
