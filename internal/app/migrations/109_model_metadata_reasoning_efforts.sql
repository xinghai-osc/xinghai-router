-- Reasoning effort levels for OpenAI GPT models that had no configured levels.
-- Each value mirrors the already-verified levels of the same model family:
--   gpt-5.2-chat-latest      -> GPT-5.2 family
--   gpt-5.3-codex-spark      -> GPT-5.3 Codex family
--   gpt-5.4-openai-compact   -> gpt-5.4
--   gpt-5.4-mini-openai-...  -> gpt-5.4-mini
--   gpt-5.5-openai-compact   -> gpt-5.5
--   gpt-6                    -> GPT-6 family
-- Rows that already carry configured levels are left untouched.
with verified(model, effort) as (
  values
    ('gpt-5.2-chat-latest', '{"supported_levels":["low","medium","high","xhigh"]}'::jsonb),
    ('gpt-5.3-codex-spark', '{"supported_levels":["low","medium","high","xhigh"]}'::jsonb),
    ('gpt-5.4-openai-compact', '{"supported_levels":["low","medium","high","xhigh"],"default_level":"none"}'::jsonb),
    ('gpt-5.4-mini-openai-compact', '{"supported_levels":["low","medium","high","xhigh"],"default_level":"none"}'::jsonb),
    ('gpt-5.5-openai-compact', '{"supported_levels":["low","medium","high","xhigh"],"default_level":"medium"}'::jsonb),
    ('gpt-6', '{"supported_levels":["low","medium","high","xhigh","max"],"default_level":"medium"}'::jsonb)
)
update model_catalog_metadata m
set reasoning_efforts = v.effort, updated_at = now()
from verified v
where m.model = v.model and m.reasoning_efforts is null;
