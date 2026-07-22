-- Convert users.id and all user_id foreign-key columns from uuid to bigint.
-- User IDs are already numeric values stored as UUID, so we extract the
-- numeric portion directly instead of generating random bigints.

begin;

-- Skip this migration on fresh installs. UUID-based IDs work fine
-- for new deployments; this conversion is only needed when migrating
-- from an older development database that used UUID user IDs.
-- If the users table is empty, the entire migration is a no-op.
do $$ begin
  if exists (select 1 from users) then

    -- 1. Drop all foreign keys pointing to users.id.
    alter table api_keys            drop constraint if exists api_keys_user_id_fkey;
    alter table request_logs        drop constraint if exists request_logs_user_id_fkey;
    alter table user_wallets        drop constraint if exists user_wallets_user_id_fkey;
    alter table wallet_ledger       drop constraint if exists wallet_ledger_user_id_fkey;
    alter table usage_records       drop constraint if exists usage_records_user_id_fkey;
    alter table quota_limits        drop constraint if exists quota_limits_user_id_fkey;
    alter table user_sessions       drop constraint if exists user_sessions_user_id_fkey;
    alter table user_permissions    drop constraint if exists user_permissions_user_id_fkey;
    alter table user_groups         drop constraint if exists user_groups_user_id_fkey;
    alter table payment_orders      drop constraint if exists payment_orders_user_id_fkey;
    alter table user_subscriptions  drop constraint if exists user_subscriptions_user_id_fkey;
    alter table subscription_orders drop constraint if exists subscription_orders_user_id_fkey;

    -- 2. Drop indexes that depend on user_id uuid columns.
    drop index if exists request_logs_user_id_idx;
    drop index if exists wallet_ledger_user_idx;
    drop index if exists usage_records_user_idx;
    drop index if exists quota_limits_scope_idx;
    drop index if exists user_sessions_user_id_idx;
    drop index if exists user_permissions_user_id_idx;
    drop index if exists user_groups_group_id_idx;
    drop index if exists payment_orders_user_idx;
    drop index if exists user_subscriptions_user_idx;
    drop index if exists user_subscriptions_active_idx;
    drop index if exists subscription_orders_user_idx;

    -- 3. Build a deterministic mapping from uuid ids to bigint.
    create temporary table user_id_mapping on commit drop as
    select id as old_id,
           ('x' || right(replace(id::text, '-', ''), 16))::bit(64)::bigint as new_id
    from users;

    alter table user_id_mapping add primary key (old_id);
    create unique index user_id_mapping_new_idx on user_id_mapping(new_id);

    -- 4. Create new bigint primary key on users.
    alter table users add column new_id bigint not null default 0;
    update users u set new_id = m.new_id from user_id_mapping m where m.old_id = u.id;
    alter table users drop column id cascade;
    alter table users rename column new_id to id;
    alter table users add primary key (id);

    -- 5. Alter every referencing column to bigint and backfill it.
    -- (api_keys, request_logs, user_wallets, wallet_ledger, usage_records,
    --  quota_limits, user_sessions, user_permissions, user_groups,
    --  payment_orders, user_subscriptions, subscription_orders)
    alter table api_keys add column new_user_id bigint not null default 0;
    update api_keys k set new_user_id = m.new_id from user_id_mapping m where m.old_id = k.user_id;
    alter table api_keys drop column user_id;
    alter table api_keys rename column new_user_id to user_id;
    alter table api_keys add foreign key (user_id) references users(id) on delete cascade;

    alter table request_logs add column new_user_id bigint;
    update request_logs l set new_user_id = m.new_id from user_id_mapping m where m.old_id = l.user_id;
    alter table request_logs drop column user_id;
    alter table request_logs rename column new_user_id to user_id;
    alter table request_logs add foreign key (user_id) references users(id) on delete set null;

    alter table user_wallets add column new_user_id bigint not null default 0;
    update user_wallets w set new_user_id = m.new_id from user_id_mapping m where m.old_id = w.user_id;
    alter table user_wallets drop column user_id;
    alter table user_wallets rename column new_user_id to user_id;
    alter table user_wallets add primary key (user_id);
    alter table user_wallets add foreign key (user_id) references users(id) on delete cascade;

    alter table wallet_ledger add column new_user_id bigint not null default 0;
    update wallet_ledger l set new_user_id = m.new_id from user_id_mapping m where m.old_id = l.user_id;
    alter table wallet_ledger drop column user_id;
    alter table wallet_ledger rename column new_user_id to user_id;
    alter table wallet_ledger add foreign key (user_id) references users(id) on delete cascade;

    alter table usage_records add column new_user_id bigint not null default 0;
    update usage_records u set new_user_id = m.new_id from user_id_mapping m where m.old_id = u.user_id;
    alter table usage_records drop column user_id;
    alter table usage_records rename column new_user_id to user_id;
    alter table usage_records add foreign key (user_id) references users(id) on delete cascade;

    alter table quota_limits add column new_user_id bigint;
    update quota_limits q set new_user_id = m.new_id from user_id_mapping m where m.old_id = q.user_id;
    alter table quota_limits drop column user_id;
    alter table quota_limits rename column new_user_id to user_id;
    alter table quota_limits add foreign key (user_id) references users(id) on delete cascade;

    alter table user_sessions add column new_user_id bigint not null default 0;
    update user_sessions s set new_user_id = m.new_id from user_id_mapping m where m.old_id = s.user_id;
    alter table user_sessions drop column user_id;
    alter table user_sessions rename column new_user_id to user_id;
    alter table user_sessions add foreign key (user_id) references users(id) on delete cascade;

    alter table user_permissions add column new_user_id bigint not null default 0;
    update user_permissions p set new_user_id = m.new_id from user_id_mapping m where m.old_id = p.user_id;
    alter table user_permissions drop column user_id;
    alter table user_permissions rename column new_user_id to user_id;
    alter table user_permissions add primary key (user_id, permission);
    alter table user_permissions add foreign key (user_id) references users(id) on delete cascade;

    alter table user_groups add column new_user_id bigint not null default 0;
    update user_groups g set new_user_id = m.new_id from user_id_mapping m where m.old_id = g.user_id;
    alter table user_groups drop column user_id;
    alter table user_groups rename column new_user_id to user_id;
    alter table user_groups add primary key (user_id, group_id);
    alter table user_groups add foreign key (user_id) references users(id) on delete cascade;

    alter table payment_orders add column new_user_id bigint not null default 0;
    update payment_orders o set new_user_id = m.new_id from user_id_mapping m where m.old_id = o.user_id;
    alter table payment_orders drop column user_id;
    alter table payment_orders rename column new_user_id to user_id;
    alter table payment_orders add foreign key (user_id) references users(id) on delete cascade;

    alter table user_subscriptions add column new_user_id bigint not null default 0;
    update user_subscriptions us set new_user_id = m.new_id from user_id_mapping m where m.old_id = us.user_id;
    alter table user_subscriptions drop column user_id;
    alter table user_subscriptions rename column new_user_id to user_id;
    alter table user_subscriptions add foreign key (user_id) references users(id) on delete cascade;

    alter table subscription_orders add column new_user_id bigint not null default 0;
    update subscription_orders o set new_user_id = m.new_id from user_id_mapping m where m.old_id = o.user_id;
    alter table subscription_orders drop column user_id;
    alter table subscription_orders rename column new_user_id to user_id;
    alter table subscription_orders add foreign key (user_id) references users(id) on delete cascade;

    -- 6. Recreate indexes.
    create index request_logs_user_id_idx         on request_logs(user_id, created_at desc);
    create index wallet_ledger_user_idx           on wallet_ledger(user_id, created_at desc);
    create index usage_records_user_idx           on usage_records(user_id, created_at desc);
    create index quota_limits_scope_idx           on quota_limits (coalesce(user_id, 0), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window");
    create index user_sessions_user_id_idx        on user_sessions(user_id);
    create index user_permissions_user_id_idx     on user_permissions(user_id);
    create index user_groups_group_id_idx         on user_groups(group_id);
    create index payment_orders_user_idx          on payment_orders(user_id, created_at desc);
    create index user_subscriptions_user_idx      on user_subscriptions(user_id, created_at desc);
    create index user_subscriptions_active_idx    on user_subscriptions(user_id, status, current_period_end);
    create index subscription_orders_user_idx     on subscription_orders(user_id, created_at desc);

    -- 7. Ensure new users get auto-incrementing bigint ids.
    create sequence if not exists users_id_seq as bigint;
    perform setval('users_id_seq', greatest(coalesce((select max(id) from users), 1), 1));
    alter table users alter column id set default nextval('users_id_seq');
    alter sequence users_id_seq owned by users.id;

  end if;
end $$;

commit;
