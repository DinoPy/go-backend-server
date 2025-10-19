-- +goose Up
CREATE TABLE notifications (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'unseen' CHECK (status IN ('unseen', 'seen', 'archived')),
	notification_type TEXT NOT NULL DEFAULT 'other' CHECK (notification_type IN ('reminder', 'due_task', 'task_completed', 'task_created', 'task_updated', 'system', 'achievement', 'other')),
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
	expires_at TIMESTAMPTZ,
	snoozed_until TIMESTAMPTZ,
	action_url TEXT,
	action_text TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_modified_at BIGINT NOT NULL DEFAULT ((EXTRACT(EPOCH FROM NOW()) * 1000)::BIGINT),
	seen_at TIMESTAMPTZ,
	archived_at TIMESTAMPTZ
);

CREATE INDEX notifications_user_id_status_idx ON notifications (user_id, status);
CREATE INDEX notifications_user_id_last_modified_at_idx ON notifications (user_id, last_modified_at DESC);
CREATE INDEX notifications_user_id_type_idx ON notifications (user_id, notification_type);
CREATE INDEX notifications_user_id_priority_idx ON notifications (user_id, priority);
CREATE INDEX notifications_expires_at_idx ON notifications (expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX notifications_user_id_snoozed_idx ON notifications (user_id, snoozed_until) WHERE snoozed_until IS NOT NULL;
CREATE INDEX notifications_snoozed_until_idx ON notifications (snoozed_until) WHERE snoozed_until IS NOT NULL;
CREATE INDEX notifications_created_at_idx ON notifications (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS notifications;
