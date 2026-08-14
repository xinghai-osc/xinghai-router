-- Redemption codes allow admins to issue codes that grant wallet balance or a
-- subscription plan when redeemed by a signed-in user. Codes are created in
-- batches (a single admin action can mint N codes with identical parameters)
-- and each individual code tracks its own redemption state.

create table if not exists redemption_codes (
  id uuid primary key,
  batch_id uuid not null,
  code text not null unique,
  reward_type text not null check (reward_type in ('balance','subscription')),
  amount numeric(20,2) not null default 0 check (amount >= 0),
  plan_id uuid references subscription_plans(id) on delete set null,
  period_days integer,
  max_uses integer not null default 1 check (max_uses >= 1),
  used_count integer not null default 0,
  expires_at timestamptz,
  enabled boolean not null default true,
  note text not null default '',
  redeemed_by bigint,
  redeemed_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists redemption_codes_batch_idx on redemption_codes(batch_id);
create index if not exists redemption_codes_code_idx on redemption_codes(code);
create index if not exists redemption_codes_enabled_idx on redemption_codes(enabled, expires_at);

create table if not exists redemption_code_redemptions (
  id uuid primary key,
  code_id uuid not null references redemption_codes(id) on delete cascade,
  user_id bigint not null references users(id) on delete cascade,
  amount numeric(20,2) not null default 0,
  plan_id uuid references subscription_plans(id) on delete set null,
  subscription_id uuid,
  created_at timestamptz not null default now()
);

create unique index if not exists redemption_code_redemptions_code_user_idx on redemption_code_redemptions(code_id, user_id);
create index if not exists redemption_code_redemptions_user_idx on redemption_code_redemptions(user_id, created_at desc);
create index if not exists redemption_code_redemptions_code_idx on redemption_code_redemptions(code_id, created_at desc);

-- The original wallet_ledger.kind CHECK (002) only allowed
-- topup/reservation/charge/release/refund/adjustment. Existing code already
-- uses 'subscription_topup'; add 'redemption' so redeeming a balance code is
-- distinguishable in the ledger.
do $$
declare
  c record;
begin
  for c in select conname from pg_constraint
           where conrelid = 'wallet_ledger'::regclass and contype = 'c'
  loop
    execute format('alter table wallet_ledger drop constraint if exists %I', c.conname);
  end loop;
  alter table wallet_ledger add constraint wallet_ledger_kind_check
    check (kind in ('topup','reservation','charge','release','refund','adjustment','subscription_topup','redemption'));
end $$;
