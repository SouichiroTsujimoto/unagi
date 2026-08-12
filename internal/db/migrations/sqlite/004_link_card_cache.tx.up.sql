CREATE TABLE IF NOT EXISTS link_card_cache (
	url_hash    TEXT PRIMARY KEY,
	url         TEXT NOT NULL,
	provider    TEXT NOT NULL,
	title       TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	image_url   TEXT NOT NULL DEFAULT '',
	site_name   TEXT NOT NULL DEFAULT '',
	html        TEXT NOT NULL DEFAULT '',
	ok          INTEGER NOT NULL DEFAULT 1,
	fetched_at  TIMESTAMP NOT NULL,
	expires_at  TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS link_card_cache_expires_at_idx
	ON link_card_cache(expires_at);
