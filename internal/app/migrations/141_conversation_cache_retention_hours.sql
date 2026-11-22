alter table site_settings add column if not exists conversation_cache_retention_hours integer not null default 24 check (conversation_cache_retention_hours between 1 and 87600);
