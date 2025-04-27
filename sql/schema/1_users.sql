-- +goose Up
CREATE TABLE users (
    id              UUID PRIMARY KEY,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL,
	created_at		TIMESTAMP	NOT NULL DEFAULT NOW(),
	updated_at		TIMESTAMP	NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE users;
