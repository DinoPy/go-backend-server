-- +goose Up
ALTER TABLE users
ADD COLUMN categories TEXT;

-- +goose Down
ALTER TABLE users
DROP COLUMN categories;
