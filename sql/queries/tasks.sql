-- name: GetTasks :many
SELECT * FROM TASKS;

-- name: GetNonCompletedTasks :many
SELECT *
FROM TASKS
WHERE is_completed = FALSE
ORDER BY user_id;

-- name: GetCompletedTasksByUUID :many
SELECT * 
FROM tasks
WHERE user_id = @user_id
	AND is_completed = TRUE
	AND (
	  sqlc.narg(start_date)::timestamp IS NULL OR completed_at >= sqlc.narg(start_date)::timestamp
	)
	AND (
	  sqlc.narg(end_date)::timestamp IS NULL OR completed_at <= sqlc.narg(end_date)::timestamp
	)
	AND (
		cardinality(@tags::text[]) = 0
		OR EXISTS (
			SELECT 1
			FROM unnest(@tags::text[]) AS tag_filter
			WHERE tag_filter ILIKE ANY (tags)
		)
	)
	  AND (
		sqlc.narg(search_query)::text IS NULL OR title ILIKE sqlc.narg(search_query)::text
	  )
	  AND (
		sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text
	  )
ORDER BY completed_at DESC;

-- name: GetActiveTaskByUUID :many
SELECT * 
FROM tasks
WHERE user_id = $1 AND is_completed = FALSE;

-- name: CreateTask :one
INSERT INTO tasks (
	id,
	title,
	description,
	created_at,
	completed_at,
	duration,
	category,
	tags,
	toggled_at,
	is_active,
	is_completed,
	user_id,
	last_modified_at
) VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8,
	$9,
	$10,
	$11,
	$12,
	$13
) RETURNING *;

-- name: ToggleTask :one
UPDATE tasks
SET 
	is_active = $2,
	toggled_at = $3,
	duration = $4,
	last_modified_at = $5
WHERE 
	id = $1
RETURNING *;

-- name: CompleteTask :one
UPDATE tasks
SET
	is_active = FALSE,
	is_completed = TRUE,
	duration = $2,
	completed_at = $3,
	last_modified_at = $4
WHERE id = $1
RETURNING *;

-- name: EditTask :one
UPDATE tasks
SET
	title = $2,
	description = $3,
	category = $4,
	tags = $5,
	last_modified_at = $6
WHERE id = $1
RETURNING *;

-- name: DeleteTask :exec
DELETE FROM tasks
WHERE id = $1;
