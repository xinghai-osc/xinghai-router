-- Per-channel request-body overrides: fields to delete from and values to set
-- in the upstream request payload. Shape:
--   {"delete": ["promptCacheKey"], "set": {"model": "gpt-4o-mini"}}
-- Applied after every built-in rewrite (stream_options, model mapping,
-- format conversion), so admin configuration is authoritative.

alter table channels add column if not exists request_overrides jsonb not null default '{}'::jsonb;
