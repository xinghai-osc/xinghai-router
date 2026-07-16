alter table users add column password_hash text;

create table user_sessions (
  id uuid primary key,
  user_id uuid not null references users(id) on delete cascade,
  token_hash text not null unique,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);

create index user_sessions_token_hash_idx on user_sessions(token_hash);
create index user_sessions_user_id_idx on user_sessions(user_id);
