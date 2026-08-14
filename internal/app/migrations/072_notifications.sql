create table if not exists notifications (
  id uuid primary key,
  title text not null,
  content text not null default '',
  enabled boolean not null default true,
  sort_order int not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists notifications_enabled_idx on notifications(enabled, sort_order, created_at);
