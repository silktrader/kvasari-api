package sqlite

const schema = `
BEGIN TRANSACTION;

CREATE TABLE
	IF NOT EXISTS users (
		id TEXT NOT NULL,
		alias TEXT NOT NULL CHECK (length ("alias") >= 5 AND length ("alias") < 16) UNIQUE,
		name TEXT,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		salt TEXT,
		created datetime NOT NULL,
		updated datetime NOT NULL,
		PRIMARY KEY ("id")
	);

CREATE UNIQUE INDEX IF NOT EXISTS "Title Index" ON "users" ("alias" ASC);

CREATE TABLE
	IF NOT EXISTS artworks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		type TEXT NOT NULL,
		picture_url TEXT NOT NULL,
		author_id TEXT NOT NULL,
		description TEXT,
		year INTEGER,
		location TEXT,
		created datetime,
		added datetime NOT NULL,
		updated datetime NOT NULL,
		deleted INTEGER DEFAULT 0 CHECK (deleted in (0, 1)),
		FOREIGN KEY (author_id) REFERENCES users (id)
	);

CREATE TABLE
	IF NOT EXISTS followers (
		follower TEXT NOT NULL,
		target TEXT NOT NULL,
		date	datetime NOT NULL,
		CONSTRAINT follower_fk FOREIGN KEY (follower) REFERENCES users (id) ON DELETE CASCADE,
		CONSTRAINT target_fk FOREIGN KEY (target) REFERENCES users (id) ON DELETE CASCADE,
		CONSTRAINT follower_target_pk PRIMARY KEY (follower, target)
	);

COMMIT;
`