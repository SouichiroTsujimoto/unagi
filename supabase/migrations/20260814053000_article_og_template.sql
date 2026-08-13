ALTER TABLE articles
    ADD COLUMN IF NOT EXISTS og_template text NOT NULL DEFAULT 'dot-dark';

ALTER TABLE articles
    DROP CONSTRAINT IF EXISTS articles_og_template_check;

ALTER TABLE articles
    ADD CONSTRAINT articles_og_template_check
    CHECK (og_template IN ('editorial', 'dot-dark'));
