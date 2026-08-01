-- Record which channel_api_keys row was used for a gateway request, so multi-key
-- channels can show the selected key name in usage/request logs.
alter table request_logs add column if not exists channel_key_id uuid references channel_api_keys(id) on delete set null;
create index if not exists request_logs_channel_key_id_idx on request_logs(channel_key_id) where channel_key_id is not null;