alter table site_settings add column if not exists invitations_enabled boolean not null default false;
alter table site_settings add column if not exists inviter_reward numeric(20,8) not null default 0 check (inviter_reward >= 0);
alter table site_settings add column if not exists invitee_reward numeric(20,8) not null default 0 check (invitee_reward >= 0);

create table if not exists invitation_codes (
  user_id bigint primary key references users(id) on delete cascade,
  code text not null unique,
  created_at timestamptz not null default now()
);
create table if not exists invitations (
  id uuid primary key,
  inviter_id bigint not null references users(id) on delete cascade,
  invitee_id bigint not null unique references users(id) on delete cascade,
  code text not null,
  inviter_reward numeric(20,8) not null default 0,
  invitee_reward numeric(20,8) not null default 0,
  created_at timestamptz not null default now(),
  check (inviter_id <> invitee_id)
);
create index if not exists invitations_inviter_idx on invitations(inviter_id, created_at desc);

alter table wallet_ledger drop constraint if exists wallet_ledger_kind_check;
alter table wallet_ledger add constraint wallet_ledger_kind_check
  check (kind in ('topup','reservation','charge','release','refund','adjustment','subscription_topup','redemption','invitation'));
