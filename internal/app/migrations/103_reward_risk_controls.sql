alter table site_settings add column if not exists reward_risk_settings jsonb not null default '{}';
alter table users add column if not exists risk_reviewed_through timestamptz not null default 'epoch';

create table if not exists risk_contexts (
  id uuid primary key,
  purpose text not null check (purpose in ('register','oauth','checkin')),
  user_id bigint references users(id) on update cascade on delete cascade,
  browser_hash text not null,
  rtc_hashes text[] not null default '{}',
  rtc_status text not null,
  oauth_nonce_hash text,
  oauth_provider text,
  consumed_at timestamptz,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);
create index if not exists risk_contexts_expiry_idx on risk_contexts(expires_at);
create unique index if not exists risk_contexts_oauth_idx on risk_contexts(oauth_nonce_hash) where oauth_nonce_hash is not null;

create table if not exists risk_observations (
  id uuid primary key,
  user_id bigint not null references users(id) on update cascade on delete cascade,
  action text not null check (action in ('register','login','checkin')),
  source_id text not null,
  http_ip_hash text not null default '',
  browser_hash text not null default '',
  rtc_hashes text[] not null default '{}',
  rtc_status text not null default 'unknown',
  inviter_id bigint references users(id) on update cascade on delete set null,
  name_snapshot text not null,
  decision text not null check (decision in ('allow','review','ban')),
  reasons jsonb not null default '[]',
  details jsonb not null default '{}',
  created_at timestamptz not null default now(),
  unique(action,user_id,source_id)
);
create index if not exists risk_observations_ip_idx on risk_observations(http_ip_hash,created_at desc) where http_ip_hash<>'';
create index if not exists risk_observations_browser_idx on risk_observations(browser_hash,created_at desc) where browser_hash<>'';
create index if not exists risk_observations_inviter_idx on risk_observations(inviter_id,created_at desc);
create index if not exists risk_observations_rtc_idx on risk_observations using gin(rtc_hashes);
create index if not exists risk_observations_user_idx on risk_observations(user_id,created_at desc);
create index if not exists risk_observations_action_idx on risk_observations(action,created_at desc);
create index if not exists risk_observations_decision_idx on risk_observations(decision,created_at desc);

create table if not exists reward_claims (
  id uuid primary key,
  source text not null check (source in ('invitation','checkin')),
  source_id text not null,
  user_id bigint not null references users(id) on update cascade on delete cascade,
  origin_user_id bigint not null references users(id) on update cascade on delete cascade,
  amount numeric(20,8) not null check(amount>=0),
  status text not null check(status in ('pending','credited','rejected','withdrawn')),
  risk_event_id uuid references risk_observations(id) on delete set null,
  reviewed_by bigint references users(id) on update cascade on delete set null,
  review_reason text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique(source,source_id,user_id)
);
create index if not exists reward_claims_status_idx on reward_claims(status,created_at desc);
create index if not exists reward_claims_user_idx on reward_claims(user_id,created_at desc);
create index if not exists reward_claims_origin_idx on reward_claims(origin_user_id,created_at desc);
alter table wallet_ledger add column if not exists reward_claim_id uuid references reward_claims(id) on delete set null;
create unique index if not exists wallet_ledger_reward_claim_idx on wallet_ledger(reward_claim_id) where reward_claim_id is not null;
alter table user_checkins add column if not exists reward_source_id uuid;
alter table user_checkins add column if not exists reward_status text not null default 'credited' check(reward_status in ('pending','credited','rejected','withdrawn'));
create unique index if not exists user_checkins_reward_source_idx on user_checkins(reward_source_id) where reward_source_id is not null;

create table if not exists risk_bans (
  id uuid primary key,
  user_id bigint not null references users(id) on update cascade on delete cascade,
  observation_id uuid references risk_observations(id) on delete set null,
  rule_version integer not null,
  details jsonb not null default '{}',
  banned_at timestamptz not null default now(),
  released_at timestamptz,
  released_by bigint references users(id) on update cascade on delete set null,
  release_reason text not null default ''
);
create unique index if not exists risk_bans_active_idx on risk_bans(user_id) where released_at is null;
create index if not exists risk_bans_created_idx on risk_bans(banned_at desc);
