-- Per-channel test model: when set, manual channel tests and scheduled health
-- checks probe the channel with a real chat completion using this model instead
-- of GET /v1/models. Empty falls back to the model-list probe.

alter table channels add column if not exists test_model text not null default '';
