-- +goose Up
CREATE INDEX idx_tasks_user_id_active ON tasks(user_id, is_completed) WHERE is_completed = FALSE;
CREATE INDEX idx_tasks_user_id_completed ON tasks(user_id, completed_at) WHERE is_completed = TRUE;
CREATE INDEX idx_tasks_category ON tasks(category);
CREATE INDEX idx_tasks_tags ON tasks USING GIN(tags);

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_user_id_active;
DROP INDEX IF EXISTS idx_tasks_user_id_completed;
DROP INDEX IF EXISTS idx_tasks_category;
DROP INDEX IF EXISTS idx_tasks_tags;
