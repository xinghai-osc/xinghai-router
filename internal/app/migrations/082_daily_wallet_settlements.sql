alter table wallet_ledger add column if not exists settlement_status text not null default 'not_applicable';
alter table wallet_ledger add column if not exists settlement_date date;
alter table wallet_ledger add column if not exists settled_at timestamptz;
alter table wallet_ledger add column if not exists settlement_batch_id uuid;
alter table wallet_ledger drop constraint if exists wallet_ledger_settlement_status_check;
alter table wallet_ledger add constraint wallet_ledger_settlement_status_check
  check (settlement_status in ('not_applicable','pending','processing','settled','failed'));

update wallet_ledger set settlement_status='settled' where kind='charge' and settlement_status='not_applicable';
update wallet_ledger set settlement_date=(created_at at time zone 'UTC')::date where kind='charge' and settlement_date is null;

create table if not exists wallet_settlements (
  id uuid primary key,
  ledger_id uuid not null unique references wallet_ledger(id) on delete cascade,
  user_id bigint not null references users(id) on delete cascade,
  request_id text not null unique,
  business_date date not null,
  amount numeric(20,8) not null check (amount >= 0),
  status text not null default 'pending' check (status in ('pending','processing','settled','failed')),
  batch_id uuid,
  settled_at timestamptz,
  error text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists wallet_settlements_user_idx on wallet_settlements(user_id, created_at desc);
create index if not exists wallet_settlements_pending_idx on wallet_settlements(business_date, status, created_at);
create index if not exists wallet_settlements_batch_idx on wallet_settlements(batch_id);

create table if not exists wallet_settlement_batches (
  id uuid primary key,
  business_date date not null unique,
  status text not null default 'processing' check (status in ('processing','settled','failed')),
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  error text not null default '',
  created_at timestamptz not null default now()
);

alter table wallet_ledger drop constraint if exists wallet_ledger_kind_check;
alter table wallet_ledger add constraint wallet_ledger_kind_check
  check (kind in ('topup','reservation','charge','release','refund','adjustment','subscription_topup','subscription_purchase','redemption','invitation','checkin'));
