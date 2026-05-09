SELECT
  `products`.`name` AS `name`
FROM
  `store`.`products`
WHERE
  LOWER(`products`.`name`) LIKE LOWER(?)
LIMIT 50
