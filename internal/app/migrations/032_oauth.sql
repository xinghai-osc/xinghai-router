create table if not exists oauth_providers (
  id text primary key,
  client_id text not null,
  client_secret_encrypted text not null,
  enabled boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists user_oauth_connections (
  id uuid primary key default gen_random_uuid(),
  -- uuid here to match users.id on fresh installs; 056 converts both to bigint.
  user_id uuid not null references users(id) on delete cascade,
  provider text not null,
  provider_user_id text not null,
  provider_username text not null default '',
  provider_avatar_url text not null default '',
  created_at timestamptz not null default now(),
  unique (provider, provider_user_id)
);

create index if not exists user_oauth_connections_user_id_idx on user_oauth_connections(user_id);
