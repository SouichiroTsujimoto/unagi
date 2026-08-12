create table if not exists link_card_cache (
  url_hash    text primary key,
  url         text not null,
  provider    text not null,
  title       text not null default '',
  description text not null default '',
  image_url   text not null default '',
  site_name   text not null default '',
  html        text not null default '',
  ok          boolean not null default true,
  fetched_at  timestamptz not null,
  expires_at  timestamptz not null
);

create index if not exists link_card_cache_expires_at_idx
  on link_card_cache(expires_at);
