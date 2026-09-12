create table if not exists exchange_rates (
  currency text primary key,
  rate_to_base numeric(20,8) not null check (rate_to_base > 0),
  enabled boolean not null default true,
  updated_at timestamptz not null default now()
);

insert into exchange_rates(currency,rate_to_base,enabled)
values('CNY',1,true)
on conflict(currency) do update set rate_to_base=1,enabled=true,updated_at=now();

alter table pricing_rules add column if not exists currency text not null default 'CNY';
update pricing_rules set currency='CNY' where currency is null or btrim(currency)='';
