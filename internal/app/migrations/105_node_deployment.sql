alter table cluster_instances add column if not exists ssh_host text not null default '';
alter table cluster_instances add column if not exists ssh_port integer not null default 22;
alter table cluster_instances add column if not exists ssh_user text not null default '';
alter table cluster_instances add column if not exists ssh_auth_method text not null default 'password';
alter table cluster_instances add column if not exists ssh_secret text not null default '';
alter table cluster_instances add column if not exists ssh_passphrase text not null default '';
alter table cluster_instances add column if not exists ssh_host_key text not null default '';
alter table cluster_instances add column if not exists deploy_path text not null default '';
alter table cluster_instances add column if not exists router_image text not null default 'ghcr.io/xinghai-osc/xinghai-router:latest';
alter table cluster_instances add column if not exists deploy_status text not null default 'idle';
alter table cluster_instances add column if not exists deploy_message text not null default '';
alter table cluster_instances add column if not exists deploy_started_at timestamptz;
alter table cluster_instances add column if not exists deploy_finished_at timestamptz;

create index if not exists cluster_instances_deploy_status_idx on cluster_instances(deploy_status, created_at desc);

alter table cluster_instances drop constraint if exists cluster_instances_ssh_port_check;
alter table cluster_instances add constraint cluster_instances_ssh_port_check check (ssh_port between 1 and 65535);
alter table cluster_instances drop constraint if exists cluster_instances_ssh_auth_method_check;
alter table cluster_instances add constraint cluster_instances_ssh_auth_method_check check (ssh_auth_method in ('password', 'private_key'));
alter table cluster_instances drop constraint if exists cluster_instances_deploy_status_check;
alter table cluster_instances add constraint cluster_instances_deploy_status_check check (deploy_status in ('idle', 'pending', 'running', 'success', 'failed'));
