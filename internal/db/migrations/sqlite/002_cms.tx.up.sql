CREATE TABLE IF NOT EXISTS articles (
	id                     INTEGER PRIMARY KEY AUTOINCREMENT,
	slug                   TEXT NOT NULL UNIQUE,
	status                 TEXT NOT NULL DEFAULT 'draft',
	published_revision_id  INTEGER,
	published_at           TIMESTAMP,
	created_at             TIMESTAMP NOT NULL,
	updated_at             TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS article_revisions (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	article_id  INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	title       TEXT NOT NULL,
	emoji       TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL DEFAULT 'tech',
	body_md     TEXT NOT NULL,
	summary     TEXT NOT NULL DEFAULT '',
	created_at  TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS article_revisions_article_id_idx
	ON article_revisions(article_id, id DESC);

CREATE TABLE IF NOT EXISTS topics (
	id    INTEGER PRIMARY KEY AUTOINCREMENT,
	name  TEXT NOT NULL UNIQUE,
	slug  TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS article_topics (
	article_id  INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	topic_id    INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
	PRIMARY KEY (article_id, topic_id)
);

CREATE TABLE IF NOT EXISTS media (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	object_key    TEXT NOT NULL UNIQUE,
	content_type  TEXT NOT NULL,
	size_bytes    INTEGER NOT NULL,
	sha256        TEXT NOT NULL,
	created_at    TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_credentials (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	credential_id    BLOB NOT NULL UNIQUE,
	public_key       BLOB NOT NULL,
	attestation_type TEXT NOT NULL DEFAULT '',
	transport        TEXT NOT NULL DEFAULT '',
	sign_count       INTEGER NOT NULL DEFAULT 0,
	backup_eligible  INTEGER NOT NULL DEFAULT 0,
	backup_state     INTEGER NOT NULL DEFAULT 0,
	aaguid           BLOB,
	display_name     TEXT NOT NULL DEFAULT '',
	created_at       TIMESTAMP NOT NULL,
	last_used_at     TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webauthn_challenges (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	challenge   TEXT NOT NULL UNIQUE,
	session_data TEXT NOT NULL,
	purpose     TEXT NOT NULL,
	expires_at  TIMESTAMP NOT NULL,
	created_at  TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	token_hash   BLOB NOT NULL UNIQUE,
	csrf_token   TEXT NOT NULL,
	expires_at   TIMESTAMP NOT NULL,
	created_at   TIMESTAMP NOT NULL,
	last_seen_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS recovery_codes (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	code_hash   BLOB NOT NULL UNIQUE,
	used_at     TIMESTAMP,
	created_at  TIMESTAMP NOT NULL
);
