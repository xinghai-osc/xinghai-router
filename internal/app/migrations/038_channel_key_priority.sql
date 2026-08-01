alter table channel_api_keys add column if not exists priority integer not null default 100;
create index if not exists idx_channel_api_keys_priority on channel_api_keys(channel_id, enabled, priority);
