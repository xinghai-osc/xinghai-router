alter table channels drop constraint if exists channels_provider_check;
alter table channels add constraint channels_provider_check check (provider in ('openai','ollama','kimi','opencode_go','anthropic','deepseek','custom'));
