-- name: GetTasks :many
SELECT * FROM TASKS;

-- name: GetNonCompletedTasks :many
SELECT *
FROM TASKS
WHERE is_completed = 0
ORDER BY user_id;

-- name: GetCompletedTasksByUUID :many
SELECT * 
FROM tasks
WHERE user_id = $1
	AND is_completed = 1
	AND completed_at >= $2
	AND completed_at <= $3
	AND (
		cardinality($4::text[]) = 0 OR
		EXISTS (
			SELECT 1 
			FROM unnest(tags) AS t
			WHERE t ILIKE ANY ($4)
		)
	)
	AND (
		$5 IS NULL OR title ILIKE $5
	)
	AND (
		$6 IS NULL OR category = $6
	);

-- name: GetActiveTaskByUUID :many
SELECT * 
FROM tasks
WHERE user_id = $1 AND is_completed = 0;

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
	is_active = 0,
	is_completed = 1,
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
