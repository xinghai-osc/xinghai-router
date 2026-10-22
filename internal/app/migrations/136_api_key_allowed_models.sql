-- Restrict the models an API key may call. An empty list means the key can
-- reach every model its group allows; entries may be exact model names or
-- wildcard patterns such as 'gpt-4*'.
alter table api_keys add column if not exists allowed_models text[] not null default '{}';
