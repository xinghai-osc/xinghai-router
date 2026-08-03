-- User-level data usage consent for conversation caching / analytics.
-- Defaults to on; a user can turn it off from their account preferences to
-- stop their requests and responses from being stored.

alter table users add column if not exists data_usage_enabled boolean not null default true;