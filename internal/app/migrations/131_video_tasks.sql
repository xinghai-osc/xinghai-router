create table if not exists video_tasks (
    id text primary key check (id ~ '^video_[A-Za-z0-9_-]+$'),
    upstream_id text not null check (length(upstream_id) between 1 and 256 and upstream_id ~ '^[A-Za-z0-9_-]+$'),
    user_id bigint not null references users(id) on update cascade on delete cascade,
    api_key_id uuid not null references api_keys(id) on delete cascade,
    workspace_id uuid not null references workspaces(id) on delete cascade,
    group_id uuid references groups(id) on delete set null,
    channel_id bigint not null,
    channel_key_id uuid,
    model text not null,
    upstream_model text not null,
    upstream_base_url text not null,
    credential_hash text not null,
    status text not null check (status in ('queued','in_progress','completed','failed')),
    created_at timestamptz not null default now(),
    unique(channel_id, credential_hash, upstream_id)
);

create index if not exists video_tasks_owner_idx on video_tasks(api_key_id, workspace_id, created_at desc);
