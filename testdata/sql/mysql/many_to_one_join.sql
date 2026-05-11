SELECT
  `customers`.`name` AS `customer_name`,
  COUNT(`orders`.`id`) AS `order_count`
FROM
  `public`.`orders`
LEFT JOIN `public`.`customers` ON `public`.`orders`.`customer_id` = `public`.`customers`.`id`
WHERE
  `orders`.`created_at` >= ?
GROUP BY
  `customers`.`name`
ORDER BY
  `order_count` DESC
LIMIT 50
