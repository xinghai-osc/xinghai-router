-- Per-plan overage policy: when a subscription's period quota is exhausted,
-- 'allow_wallet' (default) falls through to wallet billing, 'block' rejects
-- the request with 402 subscription_quota_exceeded.
alter table subscription_plans add column if not exists overage_policy text not null default 'allow_wallet' check (overage_policy in ('allow_wallet','block'));

-- Per-model quota overrides for a plan. When a row exists for a model, its
-- limits take precedence over the plan-level max_requests_per_period /
-- max_credit_per_period for that model. A null dimension inherits the
-- plan-level value. At least one limit must be set per row. Models with an
-- override draw usage only from their own requests; models without one share
-- the plan-level limits across all non-override models.
create table if not exists subscription_plan_model_quotas (
  plan_id uuid not null references subscription_plans(id) on delete cascade,
  model text not null,
  max_requests_per_period bigint,
  max_tokens_per_period bigint,
  primary key (plan_id, model)
);
