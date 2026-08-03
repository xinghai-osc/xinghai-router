alter table channel_api_keys add column if not exists failure_count integer not null default 0;
