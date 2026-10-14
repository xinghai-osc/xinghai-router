alter table model_catalog_metadata
  add column if not exists requires_data_transfer_consent boolean not null default false;

create table if not exists account_data_transfer_consents (
  user_id bigint primary key references users(id) on delete cascade,
  terms_version text not null,
  accepted_at timestamptz not null default now()
);
