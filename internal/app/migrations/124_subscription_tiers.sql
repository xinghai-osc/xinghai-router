-- Subscription catalogue rework: a subscription is now a product ("订阅") that
-- owns tiers ("等级") and each tier owns billing-period prices ("周期价格").
-- A tier carries the benefit identity (group, model whitelist, overage policy)
-- plus default per-period amounts; a price row may override the amounts for its
-- own billing period, which is how "same subscription, different period" plans
-- collapse into one tier with several prices.
--
-- Legacy data is folded into that shape: plans whose name only differs by a
-- billing-period word become one tier with one price per period, plans whose
-- benefit identity differs become separate levels of the same product, and a
-- user who held several rows of the same tier is merged into a single
-- subscription instance (period end extends, counters reset to the tier quota),
-- which is now enforced by a unique index on (user_id, tier_id).

create table if not exists subscriptions (
  id uuid primary key,
  name text not null,
  description text not null default '',
  sort_order integer not null default 0,
  enabled boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists subscriptions_name_idx on subscriptions(name);
create index if not exists subscriptions_enabled_sort_idx on subscriptions(enabled, sort_order, name);

create table if not exists subscription_tiers (
  id uuid primary key,
  subscription_id uuid not null references subscriptions(id) on delete cascade,
  level integer not null check (level >= 1),
  name text not null default '',
  description text not null default '',
  credit_amount numeric(20,8),
  group_id uuid references groups(id) on delete set null,
  model_whitelist text[] not null default '{}',
  max_requests_per_period bigint,
  max_credit_per_period numeric(20,8),
  overage_policy text not null default 'allow_wallet' check (overage_policy in ('allow_wallet','block')),
  sort_order integer not null default 0,
  enabled boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists subscription_tiers_subscription_level_idx on subscription_tiers(subscription_id, level);
create unique index if not exists subscription_tiers_subscription_name_idx on subscription_tiers(subscription_id, name);
create index if not exists subscription_tiers_subscription_sort_idx on subscription_tiers(subscription_id, sort_order, level);

create table if not exists subscription_tier_prices (
  id uuid primary key,
  tier_id uuid not null references subscription_tiers(id) on delete cascade,
  billing_period text not null check (billing_period in ('hour','day','week','month','year')),
  price numeric(20,2) not null default 0 check (price >= 0),
  currency text not null default 'CNY',
  credit_amount numeric(20,8),
  max_requests_per_period bigint,
  max_credit_per_period numeric(20,8),
  enabled boolean not null default true,
  sort_order integer not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists subscription_tier_prices_tier_period_idx on subscription_tier_prices(tier_id, billing_period);
create index if not exists subscription_tier_prices_tier_sort_idx on subscription_tier_prices(tier_id, sort_order, billing_period);

create table if not exists subscription_tier_model_quotas (
  tier_id uuid not null references subscription_tiers(id) on delete cascade,
  model text not null,
  max_requests_per_period bigint,
  max_credit_per_period numeric(20,8),
  primary key (tier_id, model)
);

-- The user_subscriptions row-level trigger (098) names plan_id, which is dropped
-- below, so it is removed here and recreated with the new column list by
-- 115_subscription_invalidation.sql.
drop trigger if exists config_invalidation_insert_delete on user_subscriptions;
drop trigger if exists config_invalidation_update on user_subscriptions;
drop trigger if exists config_invalidation_truncate on user_subscriptions;

-- ---- Fold the legacy plan catalogue into products / tiers / prices ----

create temporary table legacy_plans on commit drop as
select p.id,
       p.name,
       p.description,
       p.price,
       p.currency,
       p.billing_period,
       p.credit_amount,
       p.group_id,
       p.model_whitelist,
       p.max_requests_per_period,
       p.max_credit_per_period,
       p.overage_policy,
       p.sort_order,
       p.enabled,
       p.created_at,
       p.updated_at,
       coalesce(
         nullif(
           btrim(
             regexp_replace(
               regexp_replace(
                 p.name,
                 '(monthly|month|yearly|year|quarterly|quarter|weekly|week|daily|day|hourly|hour|月度|年度|季度|月付|年付|包月|包年|月卡|年卡|周卡|小时|月|年|季|周|日|天)',
                 '', 'gi'
               ),
               '\s+', ' ', 'g'
             ),
             ' -_·/|'
           ),
           ''
         ),
         p.name
       ) as product_name
from subscription_plans p;

create temporary table legacy_products on commit drop as
select gen_random_uuid() as product_id,
       lower(product_name) as product_key,
       min(product_name) as product_name,
       (array_agg(description order by sort_order, name))[1] as description,
       min(sort_order) as sort_order,
       bool_or(enabled) as enabled,
       min(created_at) as created_at,
       max(updated_at) as updated_at
from legacy_plans
group by lower(product_name);

create temporary table legacy_levels on commit drop as
select gen_random_uuid() as tier_id,
       g.product_id,
       g.gkey,
       g.mkey,
       g.okey,
       g.primary_plan,
       g.primary_name,
       g.min_sort,
       row_number() over (partition by g.product_id order by g.min_sort, g.min_name) as level
from (
  select prod.product_id,
         coalesce(lp.group_id::text, '') as gkey,
         array_to_string(lp.model_whitelist, ',') as mkey,
         lp.overage_policy as okey,
         (array_agg(lp.id order by lp.sort_order, lp.name))[1] as primary_plan,
         (array_agg(lp.name order by lp.sort_order, lp.name))[1] as primary_name,
         min(lp.sort_order) as min_sort,
         min(lp.name) as min_name
  from legacy_plans lp
  join legacy_products prod on prod.product_key = lower(lp.product_name)
  group by prod.product_id, coalesce(lp.group_id::text, ''), array_to_string(lp.model_whitelist, ','), lp.overage_policy
) g;

create temporary table legacy_plan_map on commit drop as
select lp.id as plan_id, l.tier_id, l.product_id, l.level
from legacy_plans lp
join legacy_products prod on prod.product_key = lower(lp.product_name)
join legacy_levels l
  on l.product_id = prod.product_id
 and l.gkey = coalesce(lp.group_id::text, '')
 and l.mkey = array_to_string(lp.model_whitelist, ',')
 and l.okey = lp.overage_policy;

insert into subscriptions(id, name, description, sort_order, enabled, created_at, updated_at)
select p.product_id, p.product_name, p.description, p.sort_order, p.enabled, p.created_at, p.updated_at
from legacy_products p;

-- A product with a single benefit identity keeps an empty tier name so the
-- console can show the product name alone; a product split into several levels
-- names each level after the plan it came from.
insert into subscription_tiers(id, subscription_id, level, name, description, credit_amount, group_id, model_whitelist, max_requests_per_period, max_credit_per_period, overage_policy, sort_order, enabled, created_at, updated_at)
select l.tier_id,
       l.product_id,
       l.level,
       case when (select count(*) from legacy_levels x where x.product_id = l.product_id) > 1 then pl.name else '' end,
       pl.description,
       pl.credit_amount,
       pl.group_id,
       pl.model_whitelist,
       pl.max_requests_per_period,
       pl.max_credit_per_period,
       pl.overage_policy,
       pl.sort_order,
       pl.enabled,
       pl.created_at,
       pl.updated_at
from legacy_levels l
join legacy_plans pl on pl.id = l.primary_plan;

-- One price per legacy plan. Amounts that differ from the tier default are kept
-- as per-period overrides, so a yearly plan that granted a year's quota keeps it
-- while sharing the tier's benefits.
insert into subscription_tier_prices(id, tier_id, billing_period, price, currency, credit_amount, max_requests_per_period, max_credit_per_period, enabled, sort_order, created_at, updated_at)
select gen_random_uuid(),
       m.tier_id,
       lp.billing_period,
       lp.price,
       lp.currency,
       case when lp.credit_amount is distinct from pl.credit_amount then lp.credit_amount else null end,
       case when lp.max_requests_per_period is distinct from pl.max_requests_per_period then lp.max_requests_per_period else null end,
       case when lp.max_credit_per_period is distinct from pl.max_credit_per_period then lp.max_credit_per_period else null end,
       lp.enabled,
       lp.sort_order,
       lp.created_at,
       lp.updated_at
from legacy_plan_map m
join legacy_plans lp on lp.id = m.plan_id
join legacy_levels l on l.tier_id = m.tier_id
join legacy_plans pl on pl.id = l.primary_plan
on conflict (tier_id, billing_period) do nothing;

insert into subscription_tier_model_quotas(tier_id, model, max_requests_per_period, max_credit_per_period)
select m.tier_id, q.model, q.max_requests_per_period, q.max_credit_per_period
from subscription_plan_model_quotas q
join legacy_plan_map m on m.plan_id = q.plan_id
on conflict (tier_id, model) do nothing;

-- ---- user_subscriptions: one instance per (user, tier) ----

alter table user_subscriptions add column if not exists tier_id uuid;
alter table user_subscriptions add column if not exists billing_period text;
alter table user_subscriptions add column if not exists price numeric(20,2);
alter table user_subscriptions add column if not exists currency text not null default 'CNY';

update user_subscriptions us
set tier_id = m.tier_id,
    billing_period = coalesce(p.billing_period, 'month'),
    price = p.price,
    currency = coalesce(p.currency, 'CNY')
from legacy_plan_map m
join subscription_plans p on p.id = m.plan_id
where us.plan_id = m.plan_id
  and (us.tier_id is null or us.billing_period is null);

-- Several rows of one tier are folded into a single instance. The survivor is
-- the row that still grants access, then the one with the furthest period end.
create temporary table legacy_subscription_merge on commit drop as
select id,
       first_value(id) over w as survivor,
       row_number() over w as rn
from user_subscriptions
where tier_id is not null
window w as (
  partition by user_id, tier_id
  order by (status = 'active' and (current_period_end is null or current_period_end > now())) desc,
           current_period_end desc nulls last,
           created_at desc
);

create temporary table legacy_merged_subscriptions on commit drop as
select survivor as id from legacy_subscription_merge group by survivor having count(*) > 1;

update subscription_orders o set subscription_id = m.survivor
from legacy_subscription_merge m
where o.subscription_id = m.id and m.rn > 1;

update reset_cards rc set subscription_id = m.survivor
from legacy_subscription_merge m
where rc.subscription_id = m.id and m.rn > 1;

update redemption_code_redemptions r set subscription_id = m.survivor
from legacy_subscription_merge m
where r.subscription_id = m.id and m.rn > 1;

update user_subscriptions us
set current_period_start = g.min_start,
    current_period_end = g.max_end,
    status = case when g.any_active then 'active' else us.status end,
    updated_at = now()
from (
  select m.survivor as id,
         min(us2.current_period_start) as min_start,
         case when bool_or(us2.current_period_end is null) then null else max(us2.current_period_end) end as max_end,
         bool_or(us2.status = 'active' and (us2.current_period_end is null or us2.current_period_end > now())) as any_active
  from legacy_subscription_merge m
  join user_subscriptions us2 on us2.id = m.id
  group by m.survivor
) g
where us.id = g.id;

delete from user_subscriptions us
using legacy_subscription_merge m
where us.id = m.id and m.rn > 1;

-- A merged instance starts a fresh quota run, which is exactly what a renewal
-- does from now on: the tier quota of the period that is currently active.
update user_subscriptions us
set remaining_requests = coalesce(pr.max_requests_per_period, t.max_requests_per_period),
    remaining_credit = coalesce(pr.max_credit_per_period, t.max_credit_per_period),
    updated_at = now()
from subscription_tiers t,
     subscription_tier_prices pr
where us.id in (select id from legacy_merged_subscriptions)
  and t.id = us.tier_id
  and pr.tier_id = t.id
  and pr.billing_period = us.billing_period;

update user_subscriptions us
set remaining_requests = t.max_requests_per_period,
    remaining_credit = t.max_credit_per_period,
    updated_at = now()
from subscription_tiers t
where us.id in (select id from legacy_merged_subscriptions)
  and t.id = us.tier_id
  and not exists (select 1 from subscription_tier_prices pr where pr.tier_id = us.tier_id and pr.billing_period = us.billing_period);

delete from user_subscription_model_usage where subscription_id in (select id from legacy_merged_subscriptions);

insert into user_subscription_model_usage(subscription_id, model, remaining_requests, remaining_credit)
select us.id, q.model, q.max_requests_per_period, q.max_credit_per_period
from user_subscriptions us
join subscription_tier_model_quotas q on q.tier_id = us.tier_id
where us.id in (select id from legacy_merged_subscriptions);

alter table user_subscriptions drop constraint if exists user_subscriptions_plan_id_fkey;
alter table user_subscriptions drop column if exists plan_id;
drop index if exists user_subscriptions_plan_idx;
delete from user_subscriptions where tier_id is null;
alter table user_subscriptions alter column tier_id set not null;
alter table user_subscriptions add constraint user_subscriptions_tier_id_fkey foreign key (tier_id) references subscription_tiers(id) on delete cascade;
create unique index if not exists user_subscriptions_user_tier_idx on user_subscriptions(user_id, tier_id);
create index if not exists user_subscriptions_tier_idx on user_subscriptions(tier_id);

-- ---- subscription_orders: keep a name/period snapshot so history survives ----

alter table subscription_orders add column if not exists tier_id uuid;
alter table subscription_orders add column if not exists plan_name text not null default '';
alter table subscription_orders add column if not exists billing_period text;

update subscription_orders o
set tier_id = m.tier_id,
    plan_name = p.name,
    billing_period = coalesce(p.billing_period, 'month')
from subscription_plans p
join legacy_plan_map m on m.plan_id = p.id
where o.plan_id = p.id and (o.tier_id is null or o.plan_name = '');

alter table subscription_orders drop constraint if exists subscription_orders_plan_id_fkey;
alter table subscription_orders drop column if exists plan_id;
alter table subscription_orders add constraint subscription_orders_tier_id_fkey foreign key (tier_id) references subscription_tiers(id) on delete set null;

-- ---- redemption codes grant a tier for a billing period ----

alter table redemption_codes drop constraint if exists redemption_codes_plan_id_fkey;
alter table redemption_code_redemptions drop constraint if exists redemption_code_redemptions_plan_id_fkey;

do $$
begin
  if exists (select 1 from information_schema.columns where table_name = 'redemption_codes' and column_name = 'plan_id') then
    alter table redemption_codes rename column plan_id to tier_id;
  end if;
  if exists (select 1 from information_schema.columns where table_name = 'redemption_code_redemptions' and column_name = 'plan_id') then
    alter table redemption_code_redemptions rename column plan_id to tier_id;
  end if;
end;
$$;

alter table redemption_codes add column if not exists tier_name text not null default '';
alter table redemption_codes add column if not exists billing_period text;

update redemption_codes rc
set billing_period = coalesce(p.billing_period, 'month'), tier_name = p.name
from subscription_plans p
where rc.tier_id = p.id and (rc.billing_period is null or rc.tier_name = '');

update redemption_codes set billing_period = 'month' where billing_period is null;

update redemption_codes rc set tier_id = m.tier_id from legacy_plan_map m where rc.tier_id = m.plan_id;
update redemption_code_redemptions r set tier_id = m.tier_id from legacy_plan_map m where r.tier_id = m.plan_id;

alter table redemption_codes add constraint redemption_codes_tier_id_fkey foreign key (tier_id) references subscription_tiers(id) on delete set null;
alter table redemption_code_redemptions add constraint redemption_code_redemptions_tier_id_fkey foreign key (tier_id) references subscription_tiers(id) on delete set null;

drop table if exists subscription_plan_model_quotas;
drop table if exists subscription_plans;
