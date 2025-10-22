-- name: LinkTaskToOccurrence :exec
INSERT INTO task_links (occurrence_id, task_id)
VALUES ($1, $2)
ON CONFLICT (occurrence_id) DO NOTHING;

-- name: GetTaskIDForOccurrence :one
SELECT task_id FROM task_links WHERE occurrence_id = $1;

-- name: GetOccurrenceForTask :one
SELECT occurrence_id FROM task_links WHERE task_id = $1;

-- name: UnlinkTaskFromOccurrence :exec
DELETE FROM task_links WHERE occurrence_id = $1;
