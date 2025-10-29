-- name: CreateSchedule :one
INSERT INTO schedules (user_id, kind, title, tz, start_local, rrule, until_local,
                       show_before_minutes, notify_offsets_min, muted_offsets_min, category)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, 0), COALESCE($9, '{2880,1440,720,360,180}')::integer[], COALESCE($10, '{}')::integer[], COALESCE($11, 'Life'))
RETURNING *;

-- name: GetActiveSchedules :many
SELECT * FROM schedules WHERE active = TRUE;

-- name: GetSchedulesByUser :many
SELECT * FROM schedules WHERE user_id = $1 AND active = TRUE ORDER BY created_at DESC;

-- name: GetScheduleByID :one
SELECT * FROM schedules WHERE id = $1;

-- name: UpdateSchedule :one
UPDATE schedules 
SET title = $2, tz = $3, start_local = $4, rrule = $5, until_local = $6,
    show_before_minutes = $7, notify_offsets_min = $8, muted_offsets_min = $9,
    category = $10, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeactivateSchedule :exec
UPDATE schedules SET active = FALSE, updated_at = NOW() WHERE id = $1;

-- name: IncrementScheduleRev :exec
UPDATE schedules SET rev = rev + 1, updated_at = NOW() WHERE id = $1;

-- name: SetLastMaterializedUntil :exec
UPDATE schedules SET last_materialized_until = $2, updated_at = NOW() WHERE id = $1;

-- name: DeleteSchedule :exec
DELETE FROM schedules WHERE id = $1;
