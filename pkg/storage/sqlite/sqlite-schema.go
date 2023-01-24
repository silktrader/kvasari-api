package sqlite

const schema = `
CREATE TABLE
	IF NOT EXISTS users (
		id TEXT NOT NULL,
		alias TEXT NOT NULL CHECK (length (alias) >= 5 AND length (alias) < 16) UNIQUE,
		name TEXT,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		salt TEXT,
		created datetime NOT NULL,
		updated datetime NOT NULL,
		PRIMARY KEY ("id")
	);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_alias ON users (alias);

CREATE TABLE
	IF NOT EXISTS artworks (
		id TEXT PRIMARY KEY,
		author_id TEXT NOT NULL,
		type TEXT NOT NULL,
		format TEXT NOT NULL,
		title TEXT,
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

CREATE TABLE
	IF NOT EXISTS bans (
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		date	datetime NOT NULL,
		CONSTRAINT source_fk FOREIGN KEY (source) REFERENCES users (id) ON DELETE CASCADE,
		CONSTRAINT target_fk FOREIGN KEY (target) REFERENCES users (id) ON DELETE CASCADE,
		CONSTRAINT source_target_pk PRIMARY KEY (source, target)
	);

CREATE TABLE
	IF NOT EXISTS artwork_feedback (
		artwork TEXT NOT NULL,
		user TEXT NOT NULL,
		reaction TEXT NOT NULL,
		date	datetime NOT NULL,
		CONSTRAINT artwork_fk FOREIGN KEY (artwork) REFERENCES artworks (id) ON DELETE CASCADE,
		CONSTRAINT user_fk FOREIGN KEY (user) REFERENCES users (id) ON DELETE CASCADE,
		CONSTRAINT artwork_user_pk PRIMARY KEY (artwork, user)
	);

CREATE TABLE
	IF NOT EXISTS artwork_comments (
		id TEXT NOT NULL PRIMARY KEY,
		artwork TEXT NOT NULL,
		user TEXT NOT NULL,
		comment TEXT NOT NULL,
		date	datetime NOT NULL,
		CONSTRAINT artwork_fk FOREIGN KEY (artwork) REFERENCES artworks (id) ON DELETE CASCADE,
		CONSTRAINT user_fk FOREIGN KEY (user) REFERENCES users (id) ON DELETE CASCADE
	);

CREATE VIEW IF NOT EXISTS deleted_artworks AS SELECT id FROM artworks WHERE deleted;

-- the BEFORE clause should prevent recursive triggers
-- there's a possible issue with date time formatting
CREATE TRIGGER IF NOT EXISTS set_user_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
BEGIN
  UPDATE users SET updated = datetime('now') WHERE id = OLD.id;
END;
`
