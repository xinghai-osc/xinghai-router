alter table subscription_plans drop constraint if exists subscription_plans_billing_period_check;
alter table subscription_plans add constraint subscription_plans_billing_period_check check (billing_period in ('hour','day','week','month','year'));
