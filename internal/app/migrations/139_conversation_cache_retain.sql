-- When conversation_cache_retain is on, the midnight cleanup keeps the cached
-- conversations instead of deleting day directories older than the retention
-- window, and the admin list/detail views read every remaining day.
alter table site_settings add column if not exists conversation_cache_retain boolean not null default false;
