alter table request_logs add column group_id uuid references groups(id) on delete set null;
create index request_logs_group_id_idx on request_logs(group_id, created_at desc);
