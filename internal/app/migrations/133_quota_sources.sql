alter table quota_limits add column if not exists source text not null default 'admin';

alter table quota_limits drop constraint if exists quota_limits_source_check;
alter table quota_limits add constraint quota_limits_source_check check (source in ('admin','self_service'));

drop index if exists quota_limits_scope_idx;
create unique index quota_limits_scope_idx on quota_limits (
    coalesce(user_id, 0),
    coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid),
    coalesce(model, ''),
    "window",
    source
);
