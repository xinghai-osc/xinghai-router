create table if not exists admin_bootstrap_state (
    singleton boolean primary key default true check (singleton),
    closed boolean not null
);

insert into admin_bootstrap_state(singleton, closed)
select true, not exists (
    select 1 from users where id=1 and email='admin@reserved.local' and name='admin'
        and role='admin' and not enabled and password_hash is null
) or exists (select 1 from users where id<>1)
on conflict (singleton) do nothing;
