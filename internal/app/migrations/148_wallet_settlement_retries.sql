alter table wallet_settlements add column if not exists retry_after timestamptz;

create index if not exists wallet_settlements_unfinished_idx
  on wallet_settlements(business_date,created_at,id) where status <> 'settled';
