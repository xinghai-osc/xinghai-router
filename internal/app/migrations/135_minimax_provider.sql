alter table channels drop constraint if exists channels_provider_check;
alter table channels add constraint channels_provider_check check (provider in ('openai','openai_chat','ollama','kimi','opencode_go','anthropic','deepseek','commandcode','jev','minimax','custom'));

alter table video_tasks add column if not exists upstream_provider text not null default 'openai';
