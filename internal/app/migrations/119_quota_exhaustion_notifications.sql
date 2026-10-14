create table if not exists quota_exhaustion_notifications (
  user_id bigint not null references users(id) on delete cascade,
  category text not null,
  event_key text not null,
  created_at timestamptz not null default now(),
  primary key(user_id, category, event_key)
);
