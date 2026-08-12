-- CMS tables are owned by the Go app via the Postgres connection string.
-- Deny PostgREST (anon / authenticated) so the public API surface is not the CMS.
alter table public.accounts enable row level security;
alter table public.articles enable row level security;
alter table public.article_revisions enable row level security;
alter table public.topics enable row level security;
alter table public.article_topics enable row level security;
alter table public.media enable row level security;
alter table public.article_stickers enable row level security;
alter table public.article_comments enable row level security;
alter table public.link_card_cache enable row level security;

do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on table
      public.accounts,
      public.articles,
      public.article_revisions,
      public.topics,
      public.article_topics,
      public.media,
      public.article_stickers,
      public.article_comments,
      public.link_card_cache
    from anon, authenticated;
  end if;
end $$;

drop table if exists public.bun_migrations;
drop table if exists public.bun_migration_locks;
