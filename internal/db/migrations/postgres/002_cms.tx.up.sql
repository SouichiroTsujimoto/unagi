CREATE TABLE IF NOT EXISTS articles (
	id                     BIGSERIAL PRIMARY KEY,
	slug                   TEXT NOT NULL UNIQUE,
	status                 TEXT NOT NULL DEFAULT 'draft',
	published_revision_id  BIGINT,
	published_at           TIMESTAMPTZ,
	created_at             TIMESTAMPTZ NOT NULL,
	updated_at             TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS article_revisions (
	id          BIGSERIAL PRIMARY KEY,
	article_id  BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	title       TEXT NOT NULL,
	emoji       TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL DEFAULT 'tech',
	body_md     TEXT NOT NULL,
	summary     TEXT NOT NULL DEFAULT '',
	created_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS article_revisions_article_id_idx
	ON article_revisions(article_id, id DESC);

CREATE TABLE IF NOT EXISTS topics (
	id    BIGSERIAL PRIMARY KEY,
	name  TEXT NOT NULL UNIQUE,
	slug  TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS article_topics (
	article_id  BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	topic_id    BIGINT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
	PRIMARY KEY (article_id, topic_id)
);

CREATE TABLE IF NOT EXISTS media (
	id            BIGSERIAL PRIMARY KEY,
	object_key    TEXT NOT NULL UNIQUE,
	content_type  TEXT NOT NULL,
	size_bytes    BIGINT NOT NULL,
	sha256        TEXT NOT NULL,
	created_at    TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_credentials (
	id               BIGSERIAL PRIMARY KEY,
	credential_id    BYTEA NOT NULL UNIQUE,
	public_key       BYTEA NOT NULL,
	attestation_type TEXT NOT NULL DEFAULT '',
	transport        TEXT NOT NULL DEFAULT '',
	sign_count       BIGINT NOT NULL DEFAULT 0,
	backup_eligible  BOOLEAN NOT NULL DEFAULT FALSE,
	backup_state     BOOLEAN NOT NULL DEFAULT FALSE,
	aaguid           BYTEA,
	display_name     TEXT NOT NULL DEFAULT '',
	created_at       TIMESTAMPTZ NOT NULL,
	last_used_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS webauthn_challenges (
	id           BIGSERIAL PRIMARY KEY,
	challenge    TEXT NOT NULL UNIQUE,
	session_data TEXT NOT NULL,
	purpose      TEXT NOT NULL,
	expires_at   TIMESTAMPTZ NOT NULL,
	created_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
	id           BIGSERIAL PRIMARY KEY,
	token_hash   BYTEA NOT NULL UNIQUE,
	csrf_token   TEXT NOT NULL,
	expires_at   TIMESTAMPTZ NOT NULL,
	created_at   TIMESTAMPTZ NOT NULL,
	last_seen_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS recovery_codes (
	id          BIGSERIAL PRIMARY KEY,
	code_hash   BYTEA NOT NULL UNIQUE,
	used_at     TIMESTAMPTZ,
	created_at  TIMESTAMPTZ NOT NULL
);
