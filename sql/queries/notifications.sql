-- name: CreateNotification :one
INSERT INTO notifications (
	id,
	user_id,
	title,
	description,
	status,
	notification_type,
	payload,
	priority,
	expires_at,
	snoozed_until,
	action_url,
	action_text,
	last_modified_at,
	seen_at,
	archived_at
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
	$15
) RETURNING *;

-- name: GetNotificationByID :one
SELECT *
FROM notifications
WHERE id = $1;

-- name: ListNotificationsByUser :many
SELECT *
FROM notifications
WHERE user_id = @user_id
  AND (sqlc.narg(statuses)::text[] IS NULL OR status = ANY(sqlc.narg(statuses)::text[]))
  AND (sqlc.narg(notification_types)::text[] IS NULL OR notification_type = ANY(sqlc.narg(notification_types)::text[]))
  AND (sqlc.narg(priorities)::text[] IS NULL OR priority = ANY(sqlc.narg(priorities)::text[]))
  AND (sqlc.narg(expired_only)::bool IS DISTINCT FROM TRUE OR (expires_at IS NOT NULL AND expires_at < NOW()))
  AND (sqlc.narg(include_snoozed)::bool IS TRUE OR snoozed_until IS NULL OR snoozed_until <= NOW())
ORDER BY created_at DESC, id DESC
LIMIT COALESCE(sqlc.narg(limit_val)::int, 50)
OFFSET COALESCE(sqlc.narg(offset_val)::int, 0);

-- name: MarkNotificationSeen :one
UPDATE notifications
SET status = 'seen',
	seen_at = COALESCE(seen_at, NOW()),
	snoozed_until = NULL,
	updated_at = NOW(),
	last_modified_at = $3
WHERE id = $1
  AND user_id = $2
RETURNING *;

-- name: ArchiveNotification :one
UPDATE notifications
SET status = 'archived',
	archived_at = NOW(),
	snoozed_until = NULL,
	updated_at = NOW(),
	last_modified_at = $3
WHERE id = $1
  AND user_id = $2
RETURNING *;

-- name: UpdateNotificationDetails :one
UPDATE notifications
SET title = $2,
	description = $3,
	notification_type = $4,
	payload = $5,
	priority = $6,
	expires_at = $7,
	action_url = $8,
	action_text = $9,
	snoozed_until = $10,
	status = $11,
	updated_at = NOW(),
	last_modified_at = $12,
	seen_at = CASE WHEN $11 = 'seen' THEN COALESCE(seen_at, NOW()) ELSE seen_at END,
	archived_at = CASE WHEN $11 = 'archived' THEN COALESCE(archived_at, NOW()) ELSE archived_at END
WHERE id = $1
RETURNING *;

-- name: CountUnseenNotifications :one
SELECT COUNT(*) as count
FROM notifications
WHERE user_id = $1 
  AND status = 'unseen'
  AND (snoozed_until IS NULL OR snoozed_until <= NOW());

-- name: MarkAllNotificationsSeen :many
UPDATE notifications
SET status = 'seen',
	seen_at = COALESCE(seen_at, NOW()),
	snoozed_until = NULL,
	updated_at = NOW(),
	last_modified_at = $2
WHERE user_id = $1 AND status = 'unseen'
RETURNING *;

-- name: ArchiveAllNotifications :many
UPDATE notifications
SET status = 'archived',
	archived_at = NOW(),
	snoozed_until = NULL,
	updated_at = NOW(),
	last_modified_at = $2
WHERE user_id = $1 AND status IN ('unseen', 'seen')
RETURNING *;

-- name: GetNotificationsByType :many
SELECT *
FROM notifications
WHERE user_id = $1 AND notification_type = $2
  AND (snoozed_until IS NULL OR snoozed_until <= NOW())
ORDER BY created_at DESC, id DESC
LIMIT COALESCE($3::int, 50)
OFFSET COALESCE($4::int, 0);

-- name: GetExpiredNotifications :many
SELECT *
FROM notifications
WHERE expires_at IS NOT NULL 
  AND expires_at < NOW()
  AND status != 'archived'
  AND (snoozed_until IS NULL OR snoozed_until <= NOW())
ORDER BY expires_at ASC;

DELETE FROM notifications
WHERE expires_at IS NOT NULL 
  AND expires_at < NOW()
  AND status = 'archived';

-- name: MarkNotificationsSeen :many
UPDATE notifications
SET status = 'seen',
	seen_at = NOW(),
	snoozed_until = NULL,
	updated_at = NOW(),
	last_modified_at = sqlc.arg(last_modified_at)
WHERE user_id = sqlc.arg(user_id)
  AND id = ANY(sqlc.arg(notification_ids)::uuid[])
  AND status != 'archived'
RETURNING *;

-- name: SnoozeNotification :one
UPDATE notifications
SET snoozed_until = $2,
	status = 'unseen',
	seen_at = NULL,
	updated_at = NOW(),
	last_modified_at = $4
WHERE id = $1
  AND user_id = $3
  AND status != 'archived'
RETURNING *;

-- name: ReleaseDueSnoozedNotifications :many
UPDATE notifications
SET snoozed_until = NULL,
	status = 'unseen',
	seen_at = NULL,
	updated_at = NOW(),
	last_modified_at = $1
WHERE snoozed_until IS NOT NULL
  AND snoozed_until <= NOW()
  AND status != 'archived'
RETURNING *;

-- name: HasNotificationForTaskStage :one
SELECT EXISTS (
	SELECT 1
	FROM notifications
	WHERE user_id = $1
	  AND notification_type = $2
	  AND payload->>'task_id' = $3
	  AND payload->>'stage' = $4
	  AND created_at > NOW() - INTERVAL '5 minutes'
) AS exists;

-- name: GetNotificationByTaskAndType :one
SELECT *
FROM notifications
WHERE user_id = $1
  AND notification_type = $2
  AND payload->>'task_id' = $3
  AND created_at > NOW() - INTERVAL '1 hour'
ORDER BY created_at DESC
LIMIT 1;
