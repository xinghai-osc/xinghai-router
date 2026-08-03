-- Conversation cache: proxied request/response bodies are stored on disk as
-- JSON files under CONVERSATION_CACHE_DIR (retained one day, cleared at
-- midnight). Only the admin toggle lives in the database.
-- An admin toggle (conversation_cache_enabled on site_settings) controls whether
-- the gateway caches conversations at all.

alter table site_settings add column if not exists conversation_cache_enabled boolean not null default false;