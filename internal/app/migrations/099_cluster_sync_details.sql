alter table cluster_syncs add column if not exists attempts integer not null default 0;
alter table cluster_syncs add column if not exists updated_at timestamptz not null default now();
create index if not exists cluster_syncs_status_idx on cluster_syncs(status, created_at);
