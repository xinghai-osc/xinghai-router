alter table exchange_rates add column if not exists points_per_unit numeric(20,8) not null default 1;
update exchange_rates set points_per_unit=rate_to_base where points_per_unit=1 and rate_to_base<>1;

alter table payment_orders add column if not exists points_amount numeric(20,8) not null default 0;
update payment_orders set points_amount=amount where points_amount=0 and amount<>0;
