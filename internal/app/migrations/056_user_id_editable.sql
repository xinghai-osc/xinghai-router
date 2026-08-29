-- Make users.id editable.
-- users.id is a primary key referenced by 13 tables. This migration:
--   1. converts users.id (and every user_id column) from uuid to bigint where
--      the pre-027 uuid schema is still in place (migration 027 skipped on
--      empty users tables),
--   2. rebuilds every user_id foreign key with ON UPDATE CASCADE so an admin-
--      edited id propagates automatically, and
--   3. guarantees users.id defaults to an auto-incrementing bigint sequence.

do $$ begin
  if (select data_type from information_schema.columns where table_name = 'users' and column_name = 'id') = 'uuid' then

    -- Fresh-install path: convert users.id and every user_id column to
    -- bigint, using sequential ids so no two uuid values can collide.
    create temporary table user_id_mapping on commit drop as
    select id as old_id, row_number() over (order by created_at, id) as new_id
    from users;
    alter table user_id_mapping add primary key (old_id);
    create unique index user_id_mapping_new_idx on user_id_mapping (new_id);

    alter table users add column new_id bigint not null default 0;
    update users u set new_id = m.new_id from user_id_mapping m where m.old_id = u.id;
    alter table users drop column id cascade;
    alter table users rename column new_id to id;
    alter table users add primary key (id);

    alter table api_keys add column new_user_id bigint not null default 0;
    update api_keys k set new_user_id = m.new_id from user_id_mapping m where m.old_id = k.user_id;
    alter table api_keys drop column user_id;
    alter table api_keys rename column new_user_id to user_id;

    alter table request_logs add column new_user_id bigint;
    update request_logs l set new_user_id = m.new_id from user_id_mapping m where m.old_id = l.user_id;
    alter table request_logs drop column user_id;
    alter table request_logs rename column new_user_id to user_id;

    alter table user_wallets add column new_user_id bigint not null default 0;
    update user_wallets w set new_user_id = m.new_id from user_id_mapping m where m.old_id = w.user_id;
    alter table user_wallets drop column user_id;
    alter table user_wallets rename column new_user_id to user_id;
    alter table user_wallets add primary key (user_id);

    alter table wallet_ledger add column new_user_id bigint not null default 0;
    update wallet_ledger l set new_user_id = m.new_id from user_id_mapping m where m.old_id = l.user_id;
    alter table wallet_ledger drop column user_id;
    alter table wallet_ledger rename column new_user_id to user_id;

    alter table usage_records add column new_user_id bigint not null default 0;
    update usage_records u set new_user_id = m.new_id from user_id_mapping m where m.old_id = u.user_id;
    alter table usage_records drop column user_id;
    alter table usage_records rename column new_user_id to user_id;

    alter table quota_limits add column new_user_id bigint;
    update quota_limits q set new_user_id = m.new_id from user_id_mapping m where m.old_id = q.user_id;
    alter table quota_limits drop column user_id;
    alter table quota_limits rename column new_user_id to user_id;

    alter table user_sessions add column new_user_id bigint not null default 0;
    update user_sessions s set new_user_id = m.new_id from user_id_mapping m where m.old_id = s.user_id;
    alter table user_sessions drop column user_id;
    alter table user_sessions rename column new_user_id to user_id;

    alter table user_permissions add column new_user_id bigint not null default 0;
    update user_permissions p set new_user_id = m.new_id from user_id_mapping m where m.old_id = p.user_id;
    alter table user_permissions drop column user_id;
    alter table user_permissions rename column new_user_id to user_id;
    alter table user_permissions add primary key (user_id, permission);

    alter table user_groups add column new_user_id bigint not null default 0;
    update user_groups g set new_user_id = m.new_id from user_id_mapping m where m.old_id = g.user_id;
    alter table user_groups drop column user_id;
    alter table user_groups rename column new_user_id to user_id;
    alter table user_groups add primary key (user_id, group_id);

    alter table payment_orders add column new_user_id bigint not null default 0;
    update payment_orders o set new_user_id = m.new_id from user_id_mapping m where m.old_id = o.user_id;
    alter table payment_orders drop column user_id;
    alter table payment_orders rename column new_user_id to user_id;

    alter table user_subscriptions add column new_user_id bigint not null default 0;
    update user_subscriptions us set new_user_id = m.new_id from user_id_mapping m where m.old_id = us.user_id;
    alter table user_subscriptions drop column user_id;
    alter table user_subscriptions rename column new_user_id to user_id;

    alter table subscription_orders add column new_user_id bigint not null default 0;
    update subscription_orders o set new_user_id = m.new_id from user_id_mapping m where m.old_id = o.user_id;
    alter table subscription_orders drop column user_id;
    alter table subscription_orders rename column new_user_id to user_id;

    alter table user_oauth_connections add column new_user_id bigint not null default 0;
    update user_oauth_connections o set new_user_id = m.new_id from user_id_mapping m where m.old_id = o.user_id;
    alter table user_oauth_connections drop column user_id;
    alter table user_oauth_connections rename column new_user_id to user_id;

    -- Recreate indexes dropped along with the converted columns. Guard against
    -- collisions with indexes that survive the column swap (e.g. group_id-only
    -- indexes whose columns are not converted).
    create index if not exists request_logs_user_id_idx         on request_logs(user_id, created_at desc);
    create index if not exists wallet_ledger_user_idx           on wallet_ledger(user_id, created_at desc);
    create index if not exists usage_records_user_idx           on usage_records(user_id, created_at desc);
    create index if not exists quota_limits_scope_idx           on quota_limits (coalesce(user_id, 0), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window");
    create index if not exists user_sessions_user_id_idx        on user_sessions(user_id);
    create index if not exists user_permissions_user_id_idx     on user_permissions(user_id);
    create index if not exists user_groups_group_id_idx         on user_groups(group_id);
    create index if not exists payment_orders_user_idx          on payment_orders(user_id, created_at desc);
    create index if not exists user_subscriptions_user_idx      on user_subscriptions(user_id, created_at desc);
    create index if not exists user_subscriptions_active_idx    on user_subscriptions(user_id, status, current_period_end);
    create index if not exists subscription_orders_user_idx     on subscription_orders(user_id, created_at desc);
    create index if not exists user_oauth_connections_user_id_idx on user_oauth_connections(user_id);

  else

    -- Migrated path: drop the constraints so they can be recreated with
    -- ON UPDATE CASCADE below (columns and indexes stay untouched).
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
    alter table user_oauth_connections drop constraint if exists user_oauth_connections_user_id_fkey;

  end if;
end $$;

-- Rebuild every user_id foreign key with ON UPDATE CASCADE so an admin-
-- edited user id propagates to all referencing tables (delete behavior
-- unchanged).
alter table api_keys add constraint api_keys_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table request_logs add constraint request_logs_user_id_fkey foreign key (user_id) references users(id) on delete set null on update cascade;
alter table user_wallets add constraint user_wallets_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table wallet_ledger add constraint wallet_ledger_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table usage_records add constraint usage_records_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table quota_limits add constraint quota_limits_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table user_sessions add constraint user_sessions_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table user_permissions add constraint user_permissions_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table user_groups add constraint user_groups_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table payment_orders add constraint payment_orders_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table user_subscriptions add constraint user_subscriptions_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table subscription_orders add constraint subscription_orders_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;
alter table user_oauth_connections add constraint user_oauth_connections_user_id_fkey foreign key (user_id) references users(id) on delete cascade on update cascade;

-- Guarantee auto-incrementing bigint ids on every schema variant.
create sequence if not exists users_id_seq as bigint;
select setval('users_id_seq', greatest(coalesce((select max(id) from users), 1), 1));
alter table users alter column id set default nextval('users_id_seq');
alter sequence users_id_seq owned by users.id;
