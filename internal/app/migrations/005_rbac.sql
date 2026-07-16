create table user_permissions (
  user_id uuid not null references users(id) on delete cascade,
  permission text not null check (permission in (
    'users.read', 'users.manage', 'keys.manage', 'channels.read', 'channels.manage',
    'logs.read', 'pricing.read', 'pricing.manage', 'audit.read', 'wallets.manage',
    'routes.manage', 'quotas.manage', 'system.manage'
  )),
  primary key (user_id, permission)
);

create index user_permissions_user_id_idx on user_permissions(user_id);
