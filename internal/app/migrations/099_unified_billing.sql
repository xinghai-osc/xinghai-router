alter table pricing_rules add column if not exists dimension_prices jsonb not null default '{}'::jsonb;
alter table usage_records add column if not exists usage_facts jsonb not null default '{}'::jsonb;
alter table usage_records add column if not exists billing_snapshot jsonb not null default '{}'::jsonb;
