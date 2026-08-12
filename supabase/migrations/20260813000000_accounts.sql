create table if not exists accounts (
  id         bigserial primary key,
  email      text not null unique,
  created_at timestamptz not null
);
