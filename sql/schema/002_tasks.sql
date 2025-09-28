-- +goose Up
CREATE TABLE tasks (
	id 			 UUID PRIMARY KEY,
	name		 TEXT NOT NULL,
	description  TEXT NOT NULL,
	created_at   DATE NOT NULL DEFAULT NOW(),
	completed_at DATE,
	duration 	 TEXT NOT NULL,
	category     TEXT NOT NULL,
	tags 		 TEXT
);

-- +goose Down
DROP TABLE tasks;
