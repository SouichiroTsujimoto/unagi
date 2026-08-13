alter table articles
  add column if not exists source_path text,
  add column if not exists source_hash text;

update articles
set source_path = 'articles/' || slug || '.md'
where source_path is null;

create unique index if not exists articles_source_path_idx
  on articles(source_path)
  where source_path is not null;

create table if not exists content_sync_runs (
  id            bigserial primary key,
  run_id        text not null unique,
  commit_sha    text not null,
  repository    text not null,
  applied_at    timestamptz not null,
  article_count integer not null default 0
);
