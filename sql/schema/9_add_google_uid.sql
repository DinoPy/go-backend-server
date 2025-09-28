-- +goose Up
ALTER TABLE users ADD COLUMN google_uid TEXT UNIQUE;

-- +goose Down
ALTER TABLE users DROP COLUMN google_uid;
