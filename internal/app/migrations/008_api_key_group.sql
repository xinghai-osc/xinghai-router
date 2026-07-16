alter table api_keys add column group_id uuid references groups(id) on delete set null;
create index api_keys_group_id_idx on api_keys(group_id);
