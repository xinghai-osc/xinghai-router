-- Remaining period-quota counters, decremented once per covered request at
-- settlement time instead of recomputed from request_logs on every read. NULL
-- means the dimension is uncapped. The counters are reset to the plan's max on
-- each activation/renewal, so the period window is the subscription period.
alter table user_subscriptions add column if not exists remaining_requests bigint;
alter table user_subscriptions add column if not exists remaining_credit numeric(20,8);

-- Per-model remaining quota for a subscription, mirroring
-- subscription_plan_model_quotas. A NULL dimension inherits the plan-level
-- counter for that model's requests; an explicit value gives the model its own
-- pool.
create table if not exists user_subscription_model_usage (
  subscription_id uuid not null references user_subscriptions(id) on delete cascade,
  model text not null,
  remaining_requests bigint,
  remaining_credit numeric(20,8),
  primary key (subscription_id, model)
);

-- Backfill counters for existing active subscriptions from the usage the old
-- enforcement aggregated for the current period, so nothing gains or loses
-- quota during the switch. Plan-level pools count requests from models without
-- an explicit override for that dimension; per-model rows cover the models
-- that declare their own pool.
update user_subscriptions us
set remaining_requests = case
      when p.max_requests_per_period is null then null
      else greatest(0, p.max_requests_per_period - (
        select count(*) from request_logs rl
        where rl.user_id = us.user_id
          and (us.current_period_start is null or rl.created_at >= us.current_period_start)
          and not exists (
            select 1 from subscription_plan_model_quotas q
            where q.plan_id = p.id and q.model = rl.model and q.max_requests_per_period is not null
          )
      ))
    end,
    remaining_credit = case
      when p.max_credit_per_period is null then null
      else greatest(0, p.max_credit_per_period - coalesce((
        select sum(ur.cost) from request_logs rl
        left join usage_records ur on ur.request_id = rl.request_id
        where rl.user_id = us.user_id
          and (us.current_period_start is null or rl.created_at >= us.current_period_start)
          and not exists (
            select 1 from subscription_plan_model_quotas q
            where q.plan_id = p.id and q.model = rl.model and q.max_credit_per_period is not null
          )
      ), 0))
    end
from subscription_plans p
where us.plan_id = p.id and us.status = 'active';

insert into user_subscription_model_usage(subscription_id, model, remaining_requests, remaining_credit)
select us.id, mq.model,
  case when mq.max_requests_per_period is null then null
    else greatest(0, mq.max_requests_per_period - (
      select count(*) from request_logs rl
      where rl.user_id = us.user_id
        and (us.current_period_start is null or rl.created_at >= us.current_period_start)
        and rl.model = mq.model
    ))
  end,
  case when mq.max_credit_per_period is null then null
    else greatest(0, mq.max_credit_per_period - coalesce((
      select sum(ur.cost) from request_logs rl
      left join usage_records ur on ur.request_id = rl.request_id
      where rl.user_id = us.user_id
        and (us.current_period_start is null or rl.created_at >= us.current_period_start)
        and rl.model = mq.model
    ), 0))
  end
from user_subscriptions us
join subscription_plans p on p.id = us.plan_id
join subscription_plan_model_quotas mq on mq.plan_id = p.id
where us.status = 'active';
