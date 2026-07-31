-- Indexes for the gateway hot path.
--
-- checkQuota aggregates request_logs by api_key_id over a rolling window on every
-- proxied request; without this index it degrades into a sequential scan as the table
-- grows.
create index if not exists request_logs_api_key_idx on request_logs(api_key_id, created_at desc);

-- channelsForModel filters channels with the jsonb existence operator (models ? $1),
-- which only the default jsonb_ops GIN opclass can serve.
create index if not exists channels_models_gin_idx on channels using gin (models);

-- The same query only ever considers enabled channels and orders by priority.
create index if not exists channels_enabled_priority_idx on channels(priority, id) where enabled;

-- subscriptionCoversModel joins active subscriptions to their plans per request.
create index if not exists user_subscriptions_plan_idx on user_subscriptions(plan_id);
