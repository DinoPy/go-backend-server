Reminders, Recurrence, and Notifications — Implementation Guide (goose + sqlc)

What you get
	•	Recurring schedules (task or reminder) with RRULE + timezone.
	•	Concrete occurrences materialized into a short horizon.
	•	Pre‑created tasks that become visible at due_at - show_before minutes.
	•	Durable notification jobs for 48/24/12/6/3‑hour reminders (or user‑chosen offsets).
	•	A planner loop (expand + upsert) and a dispatcher loop (claim + send + insert into notifications).

⸻

0) Time semantics (why timestamptz if we store UTC?)

Use timestamptz for every absolute instant (due_at, planned_send_at, created_at). It stores UTC internally, compares correctly with now() (which is timestamptz), and avoids errors when session timezones vary or during DST transitions. Use timestamp (no tz) only for local wall times that define a recurrence pattern before expansion (e.g., “every Saturday 10:00 Europe/Berlin”).
You already store times in UTC → timestamptz is the natural type for those.

⸻

1) Database migrations (goose)

Directory assumed: db/migrations.

Create a migration:
```bash
goose -dir db/migrations create reminders_and_scheduler sql
```

Edit the generated file to contain the following:
```sql
-- +goose Up
-- Create extension used for UUID defaults (safe if already installed).
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1) Convert task instants to timestamptz and add visible_from
ALTER TABLE tasks
  ALTER COLUMN created_at TYPE timestamptz USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN completed_at TYPE timestamptz USING completed_at AT TIME ZONE 'UTC',
  ALTER COLUMN due_at TYPE timestamptz USING due_at AT TIME ZONE 'UTC';

-- If you’re on PG12+, a generated column is cleanest:
ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS visible_from timestamptz
  GENERATED ALWAYS AS (
    CASE
      WHEN due_at IS NULL THEN NULL
      ELSE due_at - make_interval(mins => COALESCE(show_before_due_time, 0))
    END
  ) STORED;

CREATE INDEX IF NOT EXISTS idx_tasks_visible_from ON tasks(visible_from);

-- 2) Schedules define intent (one-off or recurring)
CREATE TABLE IF NOT EXISTS schedules (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('task','reminder')),
  title text NOT NULL,

  -- Local series definition
  tz text NOT NULL,                                   -- IANA TZ (e.g., 'Europe/Berlin')
  start_local timestamp without time zone NOT NULL,   -- local wall time seed
  rrule text,                                         -- RFC 5545; NULL => one-off
  until_local timestamp without time zone,

  -- Task-specific knobs
  show_before_minutes integer DEFAULT 0,

  -- Notification knobs (minutes before occurrence)
  notify_offsets_min integer[] DEFAULT '{2880,1440,720,360,180}',
  muted_offsets_min integer[] DEFAULT '{}',

  -- Lifecycle / planner
  active boolean NOT NULL DEFAULT TRUE,
  rev integer NOT NULL DEFAULT 1,
  last_materialized_until timestamptz
);
CREATE INDEX IF NOT EXISTS idx_schedules_active ON schedules(active);
CREATE INDEX IF NOT EXISTS idx_schedules_user ON schedules(user_id);

-- 3) Occurrences = concrete UTC instances
CREATE TABLE IF NOT EXISTS occurrences (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  schedule_id uuid NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
  occurs_at timestamptz NOT NULL,
  rev integer NOT NULL,
  UNIQUE (schedule_id, occurs_at)
);
CREATE INDEX IF NOT EXISTS idx_occurs_at ON occurrences(occurs_at);

-- 4) Durable notification jobs (send queue)
CREATE TABLE IF NOT EXISTS notification_jobs (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  schedule_id uuid REFERENCES schedules(id) ON DELETE CASCADE,
  occurrence_id uuid NOT NULL REFERENCES occurrences(id) ON DELETE CASCADE,
  offset_minutes integer NOT NULL,                    -- e.g., 2880, 1440 … 0
  planned_send_at timestamptz NOT NULL,
  sent_at timestamptz,
  canceled_at timestamptz,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (occurrence_id, offset_minutes),
  CHECK ( (sent_at IS NULL) OR (canceled_at IS NULL) )
);
CREATE INDEX IF NOT EXISTS idx_jobs_due
  ON notification_jobs (planned_send_at)
  WHERE sent_at IS NULL AND canceled_at IS NULL;

-- 5) Optional: link each occurrence to the created task to avoid duplicates
CREATE TABLE IF NOT EXISTS task_links (
  occurrence_id uuid PRIMARY KEY REFERENCES occurrences(id) ON DELETE CASCADE,
  task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_task_links_task_id ON task_links(task_id);

-- +goose Down
DROP TABLE IF EXISTS task_links;
DROP INDEX IF EXISTS idx_jobs_due;
DROP TABLE IF EXISTS notification_jobs;
DROP INDEX IF EXISTS idx_occurs_at;
DROP TABLE IF EXISTS occurrences;
DROP INDEX IF EXISTS idx_schedules_user;
DROP INDEX IF EXISTS idx_schedules_active;
DROP TABLE IF EXISTS schedules;

-- Reverse visible_from change:
ALTER TABLE tasks DROP COLUMN IF EXISTS visible_from;

-- Revert timestamptz → timestamp (values will become naive UTC)
ALTER TABLE tasks
  ALTER COLUMN due_at TYPE timestamp WITHOUT time zone USING due_at AT TIME ZONE 'UTC',
  ALTER COLUMN completed_at TYPE timestamp WITHOUT time zone USING completed_at AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE timestamp WITHOUT time zone USING created_at AT TIME ZONE 'UTC';
```

If your Postgres version doesn’t support generated columns, skip adding visible_from above and compute/maintain it in code.

Run the migration:
``` bash
source .env && goose -dir db/schema postgres "$DATABASE_URL" up .env
```

2. Create the following files.

db/queries/schedules.sql
```sql
-- name: CreateSchedule :one
INSERT INTO schedules (user_id, kind, title, tz, start_local, rrule, until_local,
                       show_before_minutes, notify_offsets_min, muted_offsets_min)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, 0), COALESCE($9, '{2880,1440,720,360,180}'), COALESCE($10, '{}'))
RETURNING *;

-- name: GetActiveSchedules :many
SELECT * FROM schedules WHERE active = TRUE;

-- name: DeactivateSchedule :exec
UPDATE schedules SET active = FALSE WHERE id = $1;

-- name: IncrementScheduleRev :exec
UPDATE schedules SET rev = rev + 1 WHERE id = $1;

-- name: SetLastMaterializedUntil :exec
UPDATE schedules SET last_materialized_until = $2 WHERE id = $1;
```

db/queries/occurrences.sql
```sql
-- name: UpsertOccurrence :one
INSERT INTO occurrences (schedule_id, occurs_at, rev)
VALUES ($1, $2, $3)
ON CONFLICT (schedule_id, occurs_at)
DO UPDATE SET rev = EXCLUDED.rev
RETURNING *;

-- name: DeleteFutureOccurrencesForSchedule :exec
DELETE FROM occurrences
WHERE schedule_id = $1 AND occurs_at > now();
```

db/queries/tasks.sql
```sql
-- name: LinkTaskToOccurrence :exec
INSERT INTO task_links (occurrence_id, task_id)
VALUES ($1, $2)
ON CONFLICT (occurrence_id) DO NOTHING;

-- name: GetTaskIDForOccurrence :one
SELECT task_id FROM task_links WHERE occurrence_id = $1;
```

db/queries/jobs.sql
```sql
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
```


db/queries/notifications.sql
```sql
-- name: InsertInboxNotification :exec
INSERT INTO notifications (id, user_id, title, description, status, notification_type,
                           payload, priority, created_at, updated_at)
VALUES (uuid_generate_v4(), $1, $2, $3, 'unseen', $4, $5, 'normal', now(), now());
```

Generate code:
```bash
sqlc generate
```

3) Planner loop (Go, using sqlc + rrule-go)

Purpose: Expand each active schedule into upcoming occurrences (e.g., next 60 days), ensure a task (for kind='task'), and schedule notification jobs (48/24/12/6/3h by default, minus any muted offsets).

Key points
	•	Expand using local wall times with the schedule’s tz, then convert to UTC instants.
	•	Use idempotent upserts for occurrences and jobs.
	•	For tasks, avoid duplicates via the task_links table: only create a task if no link exists for the occurrence.

Example outline (concise):

```go
package planner

import (
  "context"
  "time"
  "encoding/json"

  db "yourmod/internal/db"
  "github.com/teambition/rrule-go"
)

type Service struct {
  q   db.Querier        // from sqlc, e.g. *db.Queries
  ctx context.Context
  horizonDays int       // e.g., 60
}

func (s *Service) Tick(now time.Time) error {
  horizon := now.AddDate(0, 0, s.horizonDays)
  schedules, err := s.q.GetActiveSchedules(s.ctx)
  if err != nil { return err }

  for _, sch := range schedules {
    start := now.Add(-2 * time.Minute) // small backfill

    // Build iterator
    loc, _ := time.LoadLocation(sch.Tz)
    seed := time.Date(
      sch.StartLocal.Year(), sch.StartLocal.Month(), sch.StartLocal.Day(),
      sch.StartLocal.Hour(), sch.StartLocal.Minute(), sch.StartLocal.Second(),
      0, loc,
    )

    var itr *rrule.Set
    if sch.Rrule.Valid && sch.Rrule.String != "" {
      r, err := rrule.StrToRRule("DTSTART:" + seed.Format("20060102T150405Z") + "\n" + sch.Rrule.String)
      if err != nil { return err }
      set := rrule.Set{}
      set.RRule(r)
      if sch.UntilLocal.Valid {
        // optional 'UNTIL' could be in RRULE; otherwise we rely on horizon
      }
      itr = &set
    } else {
      // one-off: emulate a single occurrence at seed
      set := rrule.Set{}
      d := seed
      set.RDate(d)
      itr = &set
    }

    // Iterate and materialize
    for {
      next := nextAfterLocal(itr, start, loc)
      if next.IsZero() { break }
      if next.After(horizon) { break }

      occursAtUTC := time.Date(next.Year(), next.Month(), next.Day(),
        next.Hour(), next.Minute(), next.Second(), next.Nanosecond(), loc).UTC()

      occ, err := s.q.UpsertOccurrence(s.ctx, db.UpsertOccurrenceParams{
        ScheduleID: sch.ID, OccursAt: occursAtUTC, Rev: sch.Rev,
      })
      if err != nil { return err }

      if sch.Kind == "task" {
        // Is there already a linked task?
        _, err := s.q.GetTaskIDForOccurrence(s.ctx, occ.ID)
        if err != nil {
          // No link: create task
          title := sch.Title
          desc := "" // up to you
          task, err := s.q.InsertTask(s.ctx, db.InsertTaskParams{
            UserID: sch.UserID, Title: title, Description: desc,
            DueAt: occursAtUTC, ShowBeforeDueTime: int32(sch.ShowBeforeMinutes),
            Category: nil, Duration: nil, Priority: nil,
          })
          if err != nil { return err }

          if err := s.q.LinkTaskToOccurrence(s.ctx, db.LinkTaskToOccurrenceParams{
            OccurrenceID: occ.ID, TaskID: task.ID,
          }); err != nil { return err }
        }
      }

      // Notification jobs: offsets = notify_offsets_min \ muted_offsets_min
      offsets := effectiveOffsets(sch.NotifyOffsetsMin, sch.MutedOffsetsMin)
      for _, off := range offsets {
        sendAt := occursAtUTC.Add(-time.Duration(off) * time.Minute)

        // Small payload that helps the frontend
        pay := map[string]any{
          "schedule_id": sch.ID, "occurrence_id": occ.ID,
          "offset_minutes": off, "title": sch.Title, "kind": sch.Kind,
        }
        pj, _ := json.Marshal(pay)

        if err := s.q.UpsertNotificationJob(s.ctx, db.UpsertNotificationJobParams{
          UserID: sch.UserID, ScheduleID: &sch.ID, OccurrenceID: occ.ID,
          OffsetMinutes: int32(off), PlannedSendAt: sendAt, Payload: pj,
        }); err != nil { return err }
      }

      start = occursAtUTC // advance cursor
    }

    if err := s.q.SetLastMaterializedUntil(s.ctx, db.SetLastMaterializedUntilParams{
      ID: sch.ID, LastMaterializedUntil: horizon,
    }); err != nil { return err }
  }
  return nil
}

// Helper: offsets minus muted
func effectiveOffsets(all, muted []int32) []int {
  m := map[int32]bool{}
  for _, x := range muted { m[x] = true }
  out := []int{}
  for _, x := range all {
    if !m[x] { out = append(out, int(x)) }
  }
  return out
}

// nextAfterLocal emulates NextAfter with local-time awareness.
// For weekly/monthly rules, rrule-go will honor DTSTART and BYHOUR/MINUTE if present.
func nextAfterLocal(set *rrule.Set, start time.Time, loc *time.Location) time.Time {
  // advance from start (UTC) but we can just use Next() sequentially in this planner
  // For simplicity, call set.AllBetween and iterate; or keep a cursor per schedule.
  // In this outline, assume we call set.AllBetween and pick the next.
  return time.Time{} // implement per your style
}
```

Keep the code minimal: call the planner every minute with horizonDays := 60. A simple approach to iteration is set.AllBetween(lastCursor, horizonInLocalTZ) then range over results.

4) Dispatcher loop (Go, using sqlc)

Purpose: Claim due jobs, emit to Electron, and insert a row into your notifications inbox.
```go
package dispatch

import (
  "context"
  db "yourmod/internal/db"
  "time"
)

type Service struct {
  q db.Querier
  ctx context.Context
  batch int // e.g., 100
  sendToClient func(userID [16]byte, title, desc, typ string, payload []byte) error
}

func (s *Service) Tick() error {
  jobs, err := s.q.ClaimDueNotificationJobs(s.ctx, int32(s.batch))
  if err != nil { return err }

  for _, j := range jobs {
    // Decide notification type + text
    typ := "reminder"
    title := "Reminder"
    desc := ""
    if j.OffsetMinutes == 0 {
      // at-time fire; could be 'reminder' or 'due_task'
    }

    // Emit to client; tolerate transient errors (job already marked sent)
    _ = s.sendToClient(j.UserID.Bytes, title, desc, typ, j.Payload)

    // Insert into app inbox for persistence
    if err := s.q.InsertInboxNotification(s.ctx, db.InsertInboxNotificationParams{
      UserID: j.UserID, Title: title, Description: desc,
      NotificationType: typ, Payload: j.Payload,
    }); err != nil {
      // not fatal; continue
    }
  }
  return nil
}
```

Concurrency: run multiple dispatcher workers; FOR UPDATE SKIP LOCKED in the query makes it safe.

5) REST endpoints your Electron app can call
	•	POST /api/schedules (from your Reminder window)
```json
{
  "kind": "task",              // or "reminder"
  "title": "Submit report",
  "tz": "Europe/Berlin",
  "start_local": "2025-10-24T17:00:00",   // Chrono result in local wall time
  "rrule": null,                          // or RFC5545 string
  "until_local": null,
  "show_before_minutes": 1440,            // only used for tasks
  "notify_offsets_min": [2880,1440,720,360,180],
  "muted_offsets_min": [720]
}
```
Handler calls CreateSchedule (sqlc), returns the new schedule row.
Planner will pick it up within a minute.

PATCH /api/schedules/:id (edit)
	•	Update fields; then:
		    CancelFutureJobsForSchedule(id)
		    DeleteFutureOccurrencesForSchedule(id)
	        IncrementScheduleRev(id)
	•	Planner regenerates occurrences/jobs.
DELETE /api/schedules/:id
	•	DeactivateSchedule(id) and CancelFutureJobsForSchedule(id)


6) Electron UI expectations
	•	Task list query: show where visible_from <= now() (server or client filter).
	•	Notification preferences: map checkboxes (48/24/12/6/3h) to notify_offsets_min/muted_offsets_min.
	•	RRULE: renderer provides string; server validates (optional) with rrule-go.
	•	Offline: on reconnect, fetch recent notifications rows to show missed alerts.

7) Testing checklist
	•	Unit
    	•	RRULE expansion: weekly at 09:00 across DST.
    	•	Idempotent planner re‑runs (no dup occurrences/jobs).
    	•	Edit schedule → rev increments, future jobs canceled, re‑materialized.
    	•	visible_from = due_at - show_before.
    •	Integration
    	•	One‑off task (Fri 17:00): verify task row + 5 jobs (48/24/12/6/3h).
    	•	Weekly reminder: jobs at offset 0 for horizon window.
    	•	Dispatcher claims and inserts into notifications.

8) Operational notes
	•	Run both loops as goroutines with jitter (±5s) and context cancellation.
	•	Metrics (even log counters) help: occurrences_upserted, jobs_upserted, jobs_sent, latency_ms = now - planned_send_at.
	•	Keep horizon modest (30–90 days) to limit rows; planner is safe to re‑run.
