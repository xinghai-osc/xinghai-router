create table if not exists account_legal_consents (
  user_id bigint primary key references users(id) on delete cascade,
  terms_version text not null,
  privacy_version text not null,
  accepted_at timestamptz not null default now()
);
