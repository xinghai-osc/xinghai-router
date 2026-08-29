alter table channels drop constraint if exists channels_upstream_format_check;
alter table channels add constraint channels_upstream_format_check check (upstream_format in ('','openai','openai_chat','anthropic'));
