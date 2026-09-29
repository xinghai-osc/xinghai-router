do $body$
declare
  users_id_type text;
begin
  select data_type into users_id_type
  from information_schema.columns
  where table_schema = current_schema()
    and table_name = 'users'
    and column_name = 'id';

  if users_id_type = 'bigint' then
    execute $sql$create table if not exists clusters (
      id uuid primary key,
      name text not null unique,
      description text not null default '',
      endpoint text not null default '',
      enabled boolean not null default true,
      manager_id bigint references users(id) on delete set null,
      created_at timestamptz not null default now(),
      updated_at timestamptz not null default now()
    )$sql$;
  else
    execute $sql$create table if not exists clusters (
      id uuid primary key,
      name text not null unique,
      description text not null default '',
      endpoint text not null default '',
      enabled boolean not null default true,
      manager_id uuid references users(id) on delete set null,
      created_at timestamptz not null default now(),
      updated_at timestamptz not null default now()
    )$sql$;
  end if;

  execute $sql$create table if not exists cluster_instances (
    id uuid primary key,
    cluster_id uuid not null references clusters(id) on delete cascade,
    name text not null,
    address text not null,
    metadata jsonb not null default '{}',
    enabled boolean not null default true,
    last_seen_at timestamptz,
    created_at timestamptz not null default now(),
    unique(cluster_id,name)
  )$sql$;

  execute $sql$create table if not exists cluster_syncs (
    id uuid primary key,
    cluster_id uuid not null references clusters(id) on delete cascade,
    status text not null default 'pending' check(status in ('pending','running','success','failed')),
    message text not null default '',
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz not null default now()
  )$sql$;
end $body$;

create index if not exists cluster_instances_cluster_idx on cluster_instances(cluster_id);
create index if not exists cluster_syncs_cluster_idx on cluster_syncs(cluster_id,created_at desc);
