alter table request_logs add column if not exists client_ip text not null default '';
alter table request_logs add column if not exists user_agent text not null default '';
alter table request_logs add column if not exists error_detail text not null default '';
