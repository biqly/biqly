SELECT
  "departments"."name" AS "department_name",
  COUNT("employees"."id") AS "employee_count"
FROM
  "public"."departments"
LEFT JOIN "public"."employees" ON "public"."departments"."id" = "public"."employees"."department_id"
GROUP BY
  "departments"."name"
ORDER BY
  "employee_count" DESC
LIMIT 20
