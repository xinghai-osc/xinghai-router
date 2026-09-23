alter table user_sessions add column if not exists reauthenticated_at timestamptz;
delete from user_sessions;
