-- A user now owns a single subscription instance per tier, so the auto-renew
-- choice belongs to the purchase rather than to the instance: it is recorded on
-- the order at checkout and applied when the order is paid, which keeps starting
-- an unpaid checkout from flipping the live subscription's auto-renew switch.
alter table subscription_orders add column if not exists auto_renew boolean not null default false;
