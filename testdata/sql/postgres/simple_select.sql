SELECT
  "users"."name" AS "name",
  "users"."email" AS "email"
FROM
  "public"."users"
WHERE
  "users"."age" >= $1
LIMIT 50
