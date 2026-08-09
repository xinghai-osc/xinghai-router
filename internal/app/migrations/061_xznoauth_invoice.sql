create table if not exists invoice_settings (
  id int primary key default 1 check (id = 1),
  enabled boolean not null default false,
  base_url text not null default '',
  client_id text not null default '',
  client_secret_encrypted text not null default '',
  need_pay_tax boolean not null default false,
  updated_at timestamptz not null default now()
);

insert into invoice_settings(id) values(1) on conflict do nothing;

create table if not exists invoice_applications (
  id uuid primary key,
  user_id bigint not null references users(id) on delete cascade,
  application_id text not null,
  status text not null check (status in ('pending','approved','rejected','completed','canceled')),
  buyer_type text not null check (buyer_type in ('individual','company')),
  title text not null,
  taxpayer_id text not null default '',
  buyer_address text not null default '',
  buyer_phone text not null default '',
  buyer_bank text not null default '',
  buyer_bank_account text not null default '',
  recipient_email text not null,
  total_amount numeric(20,2) not null default 0,
  currency text not null default 'CNY',
  need_pay_tax boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists invoice_applications_app_idx on invoice_applications(application_id);
create index if not exists invoice_applications_user_idx on invoice_applications(user_id, created_at desc);

create table if not exists invoice_application_orders (
  application_id uuid not null references invoice_applications(id) on delete cascade,
  order_no text not null,
  order_type text not null check (order_type in ('payment','subscription')),
  local_order_no text not null,
  amount numeric(20,2) not null default 0,
  primary key (application_id, order_no)
);