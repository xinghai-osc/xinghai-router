alter table users add column if not exists credential_version bigint not null default 1;
alter table password_reset_tokens add column if not exists user_id bigint references users(id) on delete cascade;
alter table password_reset_tokens add column if not exists credential_version bigint;

delete from password_reset_tokens where user_id is null or credential_version is null;
alter table password_reset_tokens alter column user_id set not null;
alter table password_reset_tokens alter column credential_version set not null;
create unique index if not exists password_reset_tokens_active_user_idx on password_reset_tokens(user_id) where consumed_at is null;

create or replace function invalidate_user_reset_credentials() returns trigger language plpgsql as $$
begin
    new.credential_version := old.credential_version + 1;
    update password_reset_tokens set consumed_at=now() where user_id=old.id and consumed_at is null;
    return new;
end;
$$;
create or replace trigger users_invalidate_reset_credentials
    before update of email, password_hash, enabled, role on users
    for each row when (old.email is distinct from new.email or old.password_hash is distinct from new.password_hash
        or old.enabled is distinct from new.enabled or old.role is distinct from new.role)
    execute function invalidate_user_reset_credentials();
