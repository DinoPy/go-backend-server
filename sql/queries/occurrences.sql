-- name: UpsertOccurrence :one
INSERT INTO occurrences (schedule_id, occurs_at, rev)
VALUES ($1, $2, $3)
ON CONFLICT (schedule_id, occurs_at)
DO UPDATE SET rev = EXCLUDED.rev
RETURNING *;

-- name: DeleteFutureOccurrencesForSchedule :exec
DELETE FROM occurrences
WHERE schedule_id = $1 AND occurs_at > now();

-- name: GetOccurrencesBySchedule :many
SELECT * FROM occurrences 
WHERE schedule_id = $1 
ORDER BY occurs_at ASC;

-- name: GetOccurrencesInRange :many
SELECT * FROM occurrences 
WHERE occurs_at >= $1 AND occurs_at <= $2
ORDER BY occurs_at ASC;

-- name: DeleteOldOccurrences :exec
DELETE FROM occurrences
WHERE occurs_at < now() - INTERVAL '14 days';
