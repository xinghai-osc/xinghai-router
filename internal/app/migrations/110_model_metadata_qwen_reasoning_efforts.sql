-- Reasoning effort levels for the Qwen 3.8 cloud models, which the Model Studio
-- parameter reference lists as low / medium / xhigh with a default of xhigh.
-- qwen3.8-27b already carries the same levels; only rows without configured
-- levels are updated so manual edits stay intact.
with verified(model, effort) as (
  values
    ('qwen3.8-flash', '{"supported_levels":["low","medium","xhigh"],"default_level":"xhigh"}'::jsonb),
    ('qwen3.8-max', '{"supported_levels":["low","medium","xhigh"],"default_level":"xhigh"}'::jsonb)
)
update model_catalog_metadata m
set reasoning_efforts = v.effort, updated_at = now()
from verified v
where m.model = v.model and m.reasoning_efforts is null;
