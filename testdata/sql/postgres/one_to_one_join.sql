SELECT
  "users"."email" AS "email",
  "user_profiles"."bio" AS "bio"
FROM
  "public"."users"
LEFT JOIN "public"."user_profiles" ON "public"."users"."id" = "public"."user_profiles"."user_id"
WHERE
  "users"."is_active" = $1
LIMIT 50
