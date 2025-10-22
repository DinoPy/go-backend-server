-- +goose Up
-- Create extension used for UUID defaults (safe if already installed).
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1) Convert task instants to timestamptz and add visible_from
ALTER TABLE tasks
  ALTER COLUMN created_at TYPE timestamptz USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN completed_at TYPE timestamptz USING completed_at AT TIME ZONE 'UTC',
  ALTER COLUMN due_at TYPE timestamptz USING due_at AT TIME ZONE 'UTC';

-- Add visible_from column (computed via trigger)
ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS visible_from timestamptz;

CREATE INDEX IF NOT EXISTS idx_tasks_visible_from ON tasks(visible_from);

-- Create function to compute visible_from
CREATE OR REPLACE FUNCTION compute_visible_from() RETURNS TRIGGER AS $func$ BEGIN NEW.visible_from := CASE WHEN NEW.due_at IS NULL THEN NULL ELSE NEW.due_at - (COALESCE(NEW.show_before_due_time, 0) || ' minutes')::interval END; RETURN NEW; END; $func$ LANGUAGE plpgsql;

-- Create trigger to automatically compute visible_from
CREATE TRIGGER trigger_compute_visible_from
  BEFORE INSERT OR UPDATE ON tasks
  FOR EACH ROW
  EXECUTE FUNCTION compute_visible_from();

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
  last_materialized_until timestamptz,
  
  -- Metadata
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
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
DROP TRIGGER IF EXISTS trigger_compute_visible_from ON tasks;
DROP FUNCTION IF EXISTS compute_visible_from();
DROP INDEX IF EXISTS idx_tasks_visible_from;
ALTER TABLE tasks DROP COLUMN IF EXISTS visible_from;

-- Revert timestamptz → timestamp (values will become naive UTC)
ALTER TABLE tasks
  ALTER COLUMN due_at TYPE timestamp WITHOUT time zone USING due_at AT TIME ZONE 'UTC',
  ALTER COLUMN completed_at TYPE timestamp WITHOUT time zone USING completed_at AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE timestamp WITHOUT time zone USING created_at AT TIME ZONE 'UTC';
