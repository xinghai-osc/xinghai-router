create table if not exists user_checkins (
  user_id bigint not null references users(id) on delete cascade,
  checkin_date date not null,
  streak integer not null default 1 check (streak > 0),
  reward numeric(20,8) not null check (reward >= 0),
  created_at timestamptz not null default now(),
  primary key (user_id, checkin_date)
);
create index if not exists user_checkins_user_created_idx on user_checkins(user_id, created_at desc);

alter table wallet_ledger drop constraint if exists wallet_ledger_kind_check;
alter table wallet_ledger add constraint wallet_ledger_kind_check
  check (kind in ('topup','reservation','charge','release','refund','adjustment','subscription_topup','redemption','invitation','subscription_purchase','checkin'));
