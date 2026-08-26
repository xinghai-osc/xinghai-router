create table if not exists channel_balances (
    channel_id bigint primary key references channels(id) on delete cascade,
    key_id uuid references channel_api_keys(id) on delete set null,
    balance numeric(20,8),
    used numeric(20,8),
    total numeric(20,8),
    currency text not null default 'USD',
    supported boolean not null default false,
    error text not null default '',
    fetched_at timestamptz
);
