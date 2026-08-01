alter table channels drop constraint if exists channels_provider_check;
alter table channels add constraint channels_provider_check check (provider in ('openai','ollama','kimi','opencode_go','anthropic','custom'));

alter table channels add column if not exists upstream_path text not null default '';
alter table channels add column if not exists upstream_format text not null default '' check (upstream_format in ('','openai','anthropic'));
