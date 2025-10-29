-- +goose Up
ALTER TABLE schedules
  ADD COLUMN IF NOT EXISTS category text DEFAULT 'Life';

CREATE INDEX IF NOT EXISTS idx_schedules_category ON schedules(category);

-- +goose Down
DROP INDEX IF EXISTS idx_schedules_category;
ALTER TABLE schedules DROP COLUMN IF EXISTS category;


