drop index if exists quota_limits_scope_idx;

alter table quota_limits add column if not exists created_at timestamptz not null default now();

with duplicates as (
  select ctid, row_number() over (
    partition by coalesce(user_id, 0), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window"
    order by created_at desc, id desc
  ) as position
  from quota_limits
)
delete from quota_limits q using duplicates d
where q.ctid = d.ctid and d.position > 1;

create unique index quota_limits_scope_idx on quota_limits (
  coalesce(user_id, 0),
  coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid),
  coalesce(model, ''),
  "window"
);
