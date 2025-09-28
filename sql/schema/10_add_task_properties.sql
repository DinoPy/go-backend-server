-- +goose Up
ALTER TABLE tasks ADD COLUMN priority INTEGER;
ALTER TABLE tasks ADD COLUMN due_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN show_before_due_time INTEGER; -- minutes before due date

-- +goose Down
ALTER TABLE tasks DROP COLUMN priority;
ALTER TABLE tasks DROP COLUMN due_at;
ALTER TABLE tasks DROP COLUMN show_before_due_time;