-- name: UpsertNotificationJob :exec
INSERT INTO notification_jobs (user_id, schedule_id, occurrence_id, offset_minutes, planned_send_at, payload)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (occurrence_id, offset_minutes)
DO UPDATE SET planned_send_at = EXCLUDED.planned_send_at,
              payload = EXCLUDED.payload;

-- name: CancelFutureJobsForSchedule :exec
UPDATE notification_jobs
SET canceled_at = now()
WHERE schedule_id = $1
  AND sent_at IS NULL
  AND canceled_at IS NULL
  AND planned_send_at > now();

-- name: ClaimDueNotificationJobs :many
WITH due AS (
  SELECT id
  FROM notification_jobs
  WHERE planned_send_at <= now()
    AND sent_at IS NULL
    AND canceled_at IS NULL
  ORDER BY planned_send_at
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE notification_jobs j
SET sent_at = now()
FROM due
WHERE j.id = due.id
RETURNING j.id, j.user_id, j.schedule_id, j.occurrence_id, j.offset_minutes, j.payload, j.planned_send_at;

-- name: GetNotificationJobsByUser :many
SELECT * FROM notification_jobs 
WHERE user_id = $1 
ORDER BY planned_send_at ASC;

-- name: GetPendingNotificationJobs :many
SELECT * FROM notification_jobs 
WHERE sent_at IS NULL 
  AND canceled_at IS NULL
ORDER BY planned_send_at ASC;
