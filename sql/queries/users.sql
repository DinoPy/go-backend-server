-- name: CreateUser :one
INSERT INTO users (
	id,
	first_name,
	last_name,
	email
) VALUES (
	$1,
	$2,
	$3,
	$4
)
ON CONFLICT (email)
DO UPDATE SET id = users.id
RETURNING *;

-- name: GetUserSettings :one
SELECT categories, key_commands
FROM users
WHERE id = $1;

-- name: UpdateUserCategories :one
UPDATE users
SET
	categories = $2
WHERE
	id = $1
RETURNING *;

-- name: UpdateUserCommands :one
UPDATE users
SET
	key_commands = $2
WHERE
	id = $1
RETURNING *;
