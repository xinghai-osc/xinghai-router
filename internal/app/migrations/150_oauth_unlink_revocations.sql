create table if not exists user_oauth_revocations (
  id uuid primary key default gen_random_uuid(),
  user_id bigint not null references users(id) on delete cascade,
  provider text not null,
  provider_user_id text not null,
  revoked_at timestamptz not null default now(),
  unique (user_id, provider, provider_user_id)
);

create index if not exists user_oauth_revocations_lookup_idx
  on user_oauth_revocations(provider, provider_user_id);
