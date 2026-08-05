-- Reserve user id 1 for the bootstrap administrator so the first self-registered
-- account can never claim admin on a fresh install. The placeholder is disabled
-- and passwordless until an operator claims it via BOOTSTRAP_ADMIN_* env vars;
-- existing deployments are untouched.

begin;

do $$ begin
  if not exists (select 1 from users) then
    insert into users(id,email,name,role,enabled,password_hash)
    values (1, 'admin@reserved.local', 'admin', 'admin', false, null)
    on conflict (id) do nothing;
    insert into user_wallets(user_id) values (1) on conflict do nothing;
  end if;
end $$;

-- Keep the auto-increment sequence ahead of id 1 so registrations start at 2.
select setval('users_id_seq', greatest(coalesce((select max(id) from users), 1), 1), true);

commit;
