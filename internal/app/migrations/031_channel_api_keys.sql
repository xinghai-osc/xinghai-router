create table if not exists channel_api_keys (
    id uuid primary key default gen_random_uuid(),
    channel_id uuid not null references channels(id) on delete cascade,
    name text not null default '',
    key_encrypted text not null,
    enabled boolean not null default true,
    created_at timestamptz not null default now()
);

create index if not exists idx_channel_api_keys_channel_id on channel_api_keys(channel_id);

alter table model_routes add column if not exists hidden boolean not null default false;

insert into channel_api_keys(id, channel_id, name, key_encrypted, enabled)
select gen_random_uuid(), id, 'default', api_key, true
from channels
where api_key != ''
on conflict do nothing;

create index if not exists idx_channel_api_keys_channel_enabled on channel_api_keys(channel_id, enabled);
