-- Key usage limits: add a permanent ("total") window and a cost dimension to
-- quota_limits, so a key can be capped on requests / tokens / spend over a
-- rolling day, rolling month, or its entire lifetime.
--
-- The existing CHECK constraint only allows minute/day/month; replace it with
-- one that also accepts 'total'. 'total' rows are aggregated without a
-- created_at cutoff in checkQuota (lifetime usage, never resets).
alter table quota_limits drop constraint if exists quota_limits_window_check;
alter table quota_limits add constraint quota_limits_window_check
  check ("window" in ('minute','day','month','total'));

-- max_cost is a numeric cap on cumulative spend (sum of usage_records.cost)
-- over the same window. NULL means "no cost limit", matching the existing
-- max_requests / max_tokens semantics.
alter table quota_limits add column if not exists max_cost numeric(20,8)
  check (max_cost is null or (max_cost >= 0 and max_cost <= 1000000000000));