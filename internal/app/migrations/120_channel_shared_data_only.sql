alter table channels add column if not exists shared_data_only boolean not null default false;
