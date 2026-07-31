alter table channel_api_keys add column if not exists last_checked_at timestamptz;
alter table channel_api_keys add column if not exists last_error text;
