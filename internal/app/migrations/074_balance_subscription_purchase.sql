alter table subscription_orders drop constraint if exists subscription_orders_provider_check;
alter table subscription_orders add constraint subscription_orders_provider_check
  check (provider in ('epay','wallet'));

alter table wallet_ledger drop constraint if exists wallet_ledger_kind_check;
alter table wallet_ledger add constraint wallet_ledger_kind_check
  check (kind in ('topup','reservation','charge','release','refund','adjustment','subscription_topup','subscription_purchase','redemption','invitation'));
