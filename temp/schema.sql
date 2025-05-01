CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL
, categories TEXT, key_commands TEXT DEFAULT "{}");
CREATE TABLE tasks (
	id 			TEXT PRIMARY KEY,
	title		TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at   DATE NOT NULL,
	completed_at DATE,
	duration 	TEXT NOT NULL,
	category    TEXT NOT NULL,
	tags 		TEXT,
	toggled_at  INTEGER,
	is_active   INTEGER NOT NULL,
	is_completed INTEGER NOT NULL,
	user_id 	TEXT NOT NULL, last_modified_at INTEGER NOT NULL DEFAULT(0),
	FOREIGN KEY(user_id) REFERENCES users(id)
);
