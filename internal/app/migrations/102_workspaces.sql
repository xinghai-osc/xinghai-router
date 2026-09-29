create table if not exists workspaces (
  id uuid primary key default gen_random_uuid(),
  name text not null check (length(trim(name)) between 1 and 100),
  slug text not null unique check (slug ~ '^[a-z0-9]([a-z0-9-]{0,48}[a-z0-9])?$'),
  owner_id bigint not null references users(id) on update cascade on delete cascade,
  is_personal boolean not null default false,
  archived_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (not is_personal or archived_at is null)
);

create unique index if not exists workspaces_personal_owner_idx on workspaces(owner_id) where is_personal;
create index if not exists workspaces_owner_idx on workspaces(owner_id, created_at desc);

create table if not exists workspace_members (
  workspace_id uuid not null references workspaces(id) on delete cascade,
  user_id bigint not null references users(id) on update cascade on delete cascade,
  role text not null default 'member' check (role in ('owner','admin','member')),
  created_at timestamptz not null default now(),
  primary key (workspace_id, user_id)
);

create unique index if not exists workspace_members_owner_idx on workspace_members(workspace_id) where role='owner';
create index if not exists workspace_members_user_idx on workspace_members(user_id, created_at desc);

create or replace function workspace_create_personal() returns trigger language plpgsql as $$
declare
  personal_id uuid := gen_random_uuid();
begin
  insert into workspaces(id,name,slug,owner_id,is_personal)
  values(personal_id,'Personal workspace','personal-' || personal_id::text,new.id,true);
  insert into workspace_members(workspace_id,user_id,role) values(personal_id,new.id,'owner');
  return new;
end;
$$;

drop trigger if exists users_create_personal_workspace on users;
create trigger users_create_personal_workspace after insert on users for each row execute function workspace_create_personal();

insert into workspaces(id,name,slug,owner_id,is_personal)
select personal_id,'Personal workspace','personal-' || personal_id::text,id,true
from (select u.id,gen_random_uuid() as personal_id from users u
      where not exists(select 1 from workspaces w where w.owner_id=u.id and w.is_personal)) pending;

insert into workspace_members(workspace_id,user_id,role)
select id,owner_id,'owner' from workspaces
on conflict(workspace_id,user_id) do nothing;

create or replace function workspace_check_owner(target uuid) returns void language plpgsql as $$
declare
  owner_user bigint;
  personal boolean;
begin
  select owner_id,is_personal into owner_user,personal from workspaces where id=target for update;
  if not found then
    return;
  end if;
  if not exists(select 1 from workspace_members where workspace_id=target and user_id=owner_user and role='owner')
     or exists(select 1 from workspace_members where workspace_id=target and role='owner' and user_id<>owner_user)
     or (personal and exists(select 1 from workspace_members where workspace_id=target and user_id<>owner_user)) then
    raise exception 'workspace owner membership must be preserved' using errcode='23514';
  end if;
end;
$$;

create or replace function workspace_enforce_owner() returns trigger language plpgsql as $$
begin
  if tg_table_name='workspaces' then
    perform workspace_check_owner(new.id);
  else
    if tg_op<>'INSERT' then
      perform workspace_check_owner(old.workspace_id);
    end if;
    if tg_op<>'DELETE' then
      perform workspace_check_owner(new.workspace_id);
    end if;
  end if;
  return null;
end;
$$;

drop trigger if exists workspaces_owner_invariant on workspaces;
create constraint trigger workspaces_owner_invariant after insert or update on workspaces
  deferrable initially deferred for each row execute function workspace_enforce_owner();
drop trigger if exists workspace_members_owner_invariant on workspace_members;
create constraint trigger workspace_members_owner_invariant after insert or update or delete on workspace_members
  deferrable initially deferred for each row execute function workspace_enforce_owner();

alter table api_keys add column if not exists workspace_id uuid references workspaces(id) on delete restrict;
update api_keys k set workspace_id=w.id from workspaces w
where k.workspace_id is null and w.owner_id=k.user_id and w.is_personal;
alter table api_keys alter column workspace_id set not null;
create index if not exists api_keys_workspace_idx on api_keys(workspace_id, created_at desc);

create or replace function api_key_workspace_guard() returns trigger language plpgsql as $$
declare
  workspace_archived timestamptz;
begin
  if tg_op='UPDATE' then
    if new.workspace_id is distinct from old.workspace_id then
      raise exception 'API key workspace cannot be changed' using errcode='23514';
    end if;
    return new;
  end if;
  if new.workspace_id is null then
    select id into new.workspace_id from workspaces where owner_id=new.user_id and is_personal;
  end if;
  select archived_at into workspace_archived from workspaces where id=new.workspace_id for update;
  if not found or workspace_archived is not null then
    raise exception 'workspace access denied' using errcode='23514';
  end if;
  if not exists(select 1 from workspace_members where workspace_id=new.workspace_id and user_id=new.user_id) then
    raise exception 'workspace membership required' using errcode='23514';
  end if;
  return new;
end;
$$;

drop trigger if exists api_keys_workspace_guard on api_keys;
create trigger api_keys_workspace_guard before insert or update of workspace_id on api_keys
  for each row execute function api_key_workspace_guard();

alter table request_logs add column if not exists workspace_id uuid references workspaces(id) on delete restrict;
alter table usage_records add column if not exists workspace_id uuid references workspaces(id) on delete restrict;
update request_logs r set workspace_id=k.workspace_id from api_keys k where r.workspace_id is null and k.id=r.api_key_id;
update usage_records r set workspace_id=k.workspace_id from api_keys k where r.workspace_id is null and k.id=r.api_key_id;
update request_logs r set workspace_id=w.id from workspaces w where r.workspace_id is null and w.owner_id=r.user_id and w.is_personal;
update usage_records r set workspace_id=w.id from workspaces w where r.workspace_id is null and w.owner_id=r.user_id and w.is_personal;
create index if not exists request_logs_workspace_idx on request_logs(workspace_id, created_at desc);
create index if not exists usage_records_workspace_idx on usage_records(workspace_id, created_at desc);

create or replace function usage_assign_workspace() returns trigger language plpgsql as $$
declare
  key_workspace uuid;
begin
  if new.api_key_id is not null then
    select workspace_id into key_workspace from api_keys where id=new.api_key_id;
    if not found then
      raise exception 'API key not found' using errcode='23503';
    end if;
    if new.workspace_id is not null and new.workspace_id<>key_workspace then
      raise exception 'usage workspace does not match API key' using errcode='23514';
    end if;
    new.workspace_id := key_workspace;
  elsif new.workspace_id is null and new.user_id is not null then
    select id into new.workspace_id from workspaces where owner_id=new.user_id and is_personal;
  end if;
  return new;
end;
$$;

drop trigger if exists request_logs_assign_workspace on request_logs;
create trigger request_logs_assign_workspace before insert on request_logs for each row execute function usage_assign_workspace();
drop trigger if exists usage_records_assign_workspace on usage_records;
create trigger usage_records_assign_workspace before insert on usage_records for each row execute function usage_assign_workspace();
