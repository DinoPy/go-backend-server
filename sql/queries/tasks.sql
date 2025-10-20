-- name: GetTasks :many
SELECT * FROM tasks ORDER BY created_at ASC;

-- name: GetNonCompletedTasks :many
SELECT *
FROM tasks
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
ORDER BY created_at ASC;

-- name: GetActiveTaskByUUID :many
SELECT * 
FROM tasks
WHERE user_id = $1 AND is_completed = FALSE
ORDER BY created_at ASC;

-- name: GetTasksDueForVisibility :many
SELECT * 
FROM tasks
WHERE user_id = $1 
  AND is_completed = FALSE
  AND due_at IS NOT NULL
  AND show_before_due_time IS NOT NULL
  AND due_at - INTERVAL '1 minute' * show_before_due_time <= NOW() AT TIME ZONE 'UTC'
  AND due_at - INTERVAL '1 minute' * show_before_due_time > NOW() AT TIME ZONE 'UTC' - INTERVAL '1 minute'
ORDER BY due_at ASC;

-- name: GetTasksDueForNotifications :many
SELECT * 
FROM tasks
WHERE user_id = $1 
  AND is_completed = FALSE
  AND due_at IS NOT NULL
  AND due_at > NOW() AT TIME ZONE 'UTC'
  AND due_at <= NOW() AT TIME ZONE 'UTC' + INTERVAL '48 hours'
ORDER BY due_at ASC;

-- name: GetTasksDueForVisibilityAll :many
SELECT * 
FROM tasks
WHERE is_completed = FALSE
  AND due_at IS NOT NULL
  AND show_before_due_time IS NOT NULL
  AND due_at - INTERVAL '1 minute' * show_before_due_time <= NOW() AT TIME ZONE 'UTC'
  AND due_at - INTERVAL '1 minute' * show_before_due_time > NOW() AT TIME ZONE 'UTC' - INTERVAL '1 minute'
ORDER BY due_at ASC;

-- name: GetUpcomingTasksForNotifications :many
SELECT * 
FROM tasks
WHERE is_completed = FALSE
  AND due_at IS NOT NULL
  AND due_at > NOW() AT TIME ZONE 'UTC'
  AND due_at <= NOW() AT TIME ZONE 'UTC' + INTERVAL '48 hours'
ORDER BY due_at ASC;

-- name: GetTaskByID :one
SELECT * FROM tasks WHERE id = $1;

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
	last_modified_at,
	priority,
	due_at,
	show_before_due_time
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
	$13,
	$14,
	$15,
	$16
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
	last_modified_at = $6,
	priority = $7,
	due_at = $8,
	show_before_due_time = $9
WHERE id = $1
RETURNING *;

-- name: DeleteTask :exec
DELETE FROM tasks
WHERE id = $1;
