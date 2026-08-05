create table if not exists password_reset_tokens (
	id uuid primary key default gen_random_uuid(),
	email text not null,
	token_hash text not null,
	expires_at timestamptz not null,
	consumed_at timestamptz,
	created_at timestamptz not null default now()
);

create index if not exists password_reset_tokens_email_idx on password_reset_tokens(email, created_at desc);
create index if not exists password_reset_tokens_token_hash_idx on password_reset_tokens(token_hash);
