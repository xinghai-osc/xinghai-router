-- Allow one user to hold multiple instances of the same tier, and allow an API
-- key to opt into exactly one instance. A NULL key binding preserves the
-- existing automatic subscription selection behavior.
drop index if exists user_subscriptions_user_tier_idx;
create index if not exists user_subscriptions_user_tier_idx on user_subscriptions(user_id, tier_id, status, current_period_end);

alter table api_keys add column if not exists subscription_id uuid references user_subscriptions(id) on delete set null;
create index if not exists api_keys_subscription_id_idx on api_keys(subscription_id) where subscription_id is not null;
