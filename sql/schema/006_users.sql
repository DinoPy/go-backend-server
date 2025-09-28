-- +goose Up
ALTER TABLE users 
ADD COLUMN key_commands TEXT DEFAULT '{}';

-- +goose Down
ALTER TABLE users
DROP COLUMN key_commands;
