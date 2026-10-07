-- Correction: the official OpenAI model page for gpt-5.2-chat-latest exists and
-- documents no reasoning_effort support, while gpt-5.2.md on the same site lists
-- "none (default), low, medium, high and xhigh". A chat-only variant therefore
-- must not advertise reasoning levels, so the value added by migration 109 is
-- removed. The update only clears exactly that value, so any manual edit made
-- afterwards is preserved.
update model_catalog_metadata
set reasoning_efforts = null, updated_at = now()
where model = 'gpt-5.2-chat-latest'
  and reasoning_efforts = '{"supported_levels":["low","medium","high","xhigh"]}'::jsonb;
