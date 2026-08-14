-- Per-channel User-Agent pool: when a channel has entries, the gateway sends one
-- of them as the upstream User-Agent header instead of the default client UA.
alter table channels add column if not exists ua_pool jsonb not null default '[]'::jsonb;
