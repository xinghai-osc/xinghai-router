alter table model_catalog_metadata
  add column if not exists name text not null default '',
  add column if not exists owned_by text not null default '',
  add column if not exists max_output_tokens bigint,
  add column if not exists reasoning_efforts jsonb,
  add column if not exists api_capabilities jsonb;

alter table model_catalog_metadata
  drop constraint if exists model_catalog_metadata_max_output_tokens_check;
alter table model_catalog_metadata
  add constraint model_catalog_metadata_max_output_tokens_check
  check (max_output_tokens is null or max_output_tokens > 0);
