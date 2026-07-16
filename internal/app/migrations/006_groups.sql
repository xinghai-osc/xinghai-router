create table groups (
  id uuid primary key,
  name text not null unique,
  created_at timestamptz not null default now()
);

create table user_groups (
  user_id uuid not null references users(id) on delete cascade,
  group_id uuid not null references groups(id) on delete cascade,
  primary key (user_id, group_id)
);

create table channel_groups (
  channel_id uuid not null references channels(id) on delete cascade,
  group_id uuid not null references groups(id) on delete cascade,
  primary key (channel_id, group_id)
);

create index user_groups_group_id_idx on user_groups(group_id);
create index channel_groups_group_id_idx on channel_groups(group_id);
