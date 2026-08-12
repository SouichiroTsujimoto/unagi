create table if not exists articles (
  id                    bigserial primary key,
  slug                  text not null unique,
  status                text not null default 'draft',
  published_revision_id bigint,
  published_at          timestamptz,
  created_at            timestamptz not null,
  updated_at            timestamptz not null
);

create table if not exists article_revisions (
  id         bigserial primary key,
  article_id bigint not null references articles(id) on delete cascade,
  title      text not null,
  emoji      text not null default '',
  type       text not null default 'tech',
  body_md    text not null,
  summary    text not null default '',
  created_at timestamptz not null
);

create index if not exists article_revisions_article_id_idx
  on article_revisions(article_id, id desc);

create table if not exists topics (
  id   bigserial primary key,
  name text not null unique,
  slug text not null unique
);

create table if not exists article_topics (
  article_id bigint not null references articles(id) on delete cascade,
  topic_id   bigint not null references topics(id) on delete cascade,
  primary key (article_id, topic_id)
);

create table if not exists media (
  id           bigserial primary key,
  object_key   text not null unique,
  content_type text not null,
  size_bytes   bigint not null,
  sha256       text not null,
  created_at   timestamptz not null
);
