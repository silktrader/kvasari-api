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

CREATE UNIQUE INDEX IF NOT EXISTS "Alias Index" ON "users" ("alias" ASC);

CREATE TABLE
	IF NOT EXISTS artwork (
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
		FOREIGN KEY (author_id) REFERENCES users (id)
	);

COMMIT;
`
