alter table site_settings
  add column if not exists request_audit_enabled boolean not null default true,
  add column if not exists request_audit_store_mode text not null default 'hash' check (request_audit_store_mode in ('none','hash','excerpt')),
  add column if not exists request_audit_retention_days integer not null default 30 check (request_audit_retention_days between 1 and 3650),
  add column if not exists content_policy_mode text not null default 'off' check (content_policy_mode in ('off','audit','block'));

create table if not exists content_policy_rules (
  id uuid primary key,
  name text not null,
  term text not null,
  action text not null default 'block' check (action in ('block','audit')),
  case_sensitive boolean not null default false,
  enabled boolean not null default true,
  priority integer not null default 100,
  created_by bigint references users(id) on delete set null,
  updated_by bigint references users(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists content_policy_rules_enabled_idx on content_policy_rules(enabled, priority, created_at);

create table if not exists request_content_audits (
  id uuid primary key,
  request_id text not null unique,
  user_id bigint references users(id) on delete set null,
  api_key_id uuid references api_keys(id) on delete set null,
  model text not null default '',
  endpoint text not null default '',
  decision text not null check (decision in ('allow','audit','block')),
  matched_rule_ids text[] not null default '{}',
  request_bytes integer not null default 0,
  content_length integer not null default 0,
  content_hash text not null default '',
  excerpt text not null default '',
  created_at timestamptz not null default now()
);
create index if not exists request_content_audits_created_idx on request_content_audits(created_at desc);
create index if not exists request_content_audits_decision_idx on request_content_audits(decision, created_at desc);
create index if not exists request_content_audits_user_idx on request_content_audits(user_id, created_at desc);
