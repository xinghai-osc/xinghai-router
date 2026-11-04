-- Subscription instances can be upgraded to a higher tier of the same product
-- without buying a second parallel instance. The upgrade is recorded as its own
-- order kind so history can distinguish it from a new purchase or a renewal.
alter table subscription_orders drop constraint if exists subscription_orders_period_kind_check;
alter table subscription_orders add constraint subscription_orders_period_kind_check
  check (period_kind in ('new','renewal','upgrade'));
