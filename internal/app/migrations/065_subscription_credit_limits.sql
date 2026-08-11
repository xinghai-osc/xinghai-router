-- Rename the per-period token limit to a per-period credit (cost) limit. The
-- new column stores the total billed cost (per pricing rules) that the
-- subscription may cover within one period; requests beyond it fall through
-- to wallet billing or are rejected per overage_policy.
alter table subscription_plans rename column max_tokens_per_period to max_credit_per_period;
alter table subscription_plans alter column max_credit_per_period type numeric(20,8) using max_credit_per_period::numeric;

alter table subscription_plan_model_quotas rename column max_tokens_per_period to max_credit_per_period;
alter table subscription_plan_model_quotas alter column max_credit_per_period type numeric(20,8) using max_credit_per_period::numeric;
