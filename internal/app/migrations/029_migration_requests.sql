create table if not exists migration_requests (
    id uuid primary key default gen_random_uuid(),
    status text not null default 'running',
    source_driver text not null default 'mysql',
    step text not null default '',
    current int not null default 0,
    total int not null default 0,
    detail text not null default '',
    error text not null default '',
    started_at timestamptz not null default now(),
    finished_at timestamptz,
    created_at timestamptz not null default now()
);
