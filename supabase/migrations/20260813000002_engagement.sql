create table if not exists article_stickers (
  id           bigserial primary key,
  article_id   bigint not null references articles(id) on delete cascade,
  kind         text not null,
  value        text not null,
  x            double precision not null,
  y            double precision not null,
  visitor_hash bytea,
  x_user_id    text,
  username     text not null default '',
  display_name text not null default '',
  created_at   timestamptz not null
);

create index if not exists article_stickers_article_id_idx
  on article_stickers(article_id, id asc);

create index if not exists article_stickers_article_visitor_idx
  on article_stickers(article_id, visitor_hash);

create table if not exists article_comments (
  id           bigserial primary key,
  article_id   bigint not null references articles(id) on delete cascade,
  body         text not null,
  status       text not null default 'visible',
  x_user_id    text,
  username     text not null default '',
  display_name text not null default '',
  avatar_url   text not null default '',
  created_at   timestamptz not null,
  updated_at   timestamptz not null
);

create index if not exists article_comments_article_status_idx
  on article_comments(article_id, status, created_at asc);
