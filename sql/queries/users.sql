-- name: CreateUser :one
INSERT INTO users (
	first_name,
	last_name,
	email,
	google_uid
) VALUES (
	$1,
	$2,
	$3,
	$4
)
ON CONFLICT (email)
DO NOTHING
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByGoogleUID :one
SELECT * FROM users WHERE google_uid = $1;

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
