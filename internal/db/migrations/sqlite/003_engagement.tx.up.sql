CREATE TABLE IF NOT EXISTS article_stickers (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	article_id    INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	kind          TEXT NOT NULL,
	value         TEXT NOT NULL,
	x             REAL NOT NULL,
	y             REAL NOT NULL,
	visitor_hash  BLOB,
	x_user_id     TEXT,
	username      TEXT NOT NULL DEFAULT '',
	display_name  TEXT NOT NULL DEFAULT '',
	created_at    TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS article_stickers_article_id_idx
	ON article_stickers(article_id, id ASC);

CREATE INDEX IF NOT EXISTS article_stickers_article_visitor_idx
	ON article_stickers(article_id, visitor_hash);

CREATE TABLE IF NOT EXISTS article_comments (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	article_id    INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	body          TEXT NOT NULL,
	status        TEXT NOT NULL DEFAULT 'visible',
	x_user_id     TEXT,
	username      TEXT NOT NULL DEFAULT '',
	display_name  TEXT NOT NULL DEFAULT '',
	avatar_url    TEXT NOT NULL DEFAULT '',
	created_at    TIMESTAMP NOT NULL,
	updated_at    TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS article_comments_article_status_idx
	ON article_comments(article_id, status, created_at ASC);
