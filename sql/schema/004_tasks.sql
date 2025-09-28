-- +goose Up
DROP TABLE tasks;
CREATE TABLE tasks (
	id 			 UUID PRIMARY KEY,
	title		 TEXT NOT NULL,
	description  TEXT NOT NULL,
	created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
	completed_at TIMESTAMP,
	duration 	 TEXT NOT NULL,
	category     TEXT NOT NULL,
	tags 		 TEXT[],
	toggled_at   BIGINT,
	is_active    BOOLEAN NOT NULL,
	is_completed BOOLEAN NOT NULL,
	user_id 	 UUID NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

-- +goose Down
DROP TABLE TASKS;
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


