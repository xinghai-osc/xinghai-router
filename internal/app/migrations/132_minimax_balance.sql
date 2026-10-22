create table if not exists minimax_balance_credentials (
    channel_id bigint primary key references channels(id) on delete cascade,
    cookie_encrypted text not null,
    group_id text not null,
    updated_at timestamptz not null default now()
);
