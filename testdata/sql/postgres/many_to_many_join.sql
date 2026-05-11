SELECT
  "students"."name" AS "student_name",
  "courses"."title" AS "course_title"
FROM
  "public"."students"
LEFT JOIN "public"."enrollments" ON "public"."students"."id" = "public"."enrollments"."student_id"
LEFT JOIN "public"."courses" ON "public"."enrollments"."course_id" = "public"."courses"."id"
WHERE
  "courses"."credits" >= $1
ORDER BY
  "students"."name" ASC
LIMIT 100
