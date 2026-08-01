alter table request_logs add column if not exists subscription_covered boolean not null default false;
