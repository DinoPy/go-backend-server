-- +goose Up
ALTER TABLE tasks ADD COLUMN
last_modified_at BIGINT NOT NULL DEFAULT(0);

-- +goose Down
ALTER TABLE TASKS
DROP COLUMN last_modified_at;
