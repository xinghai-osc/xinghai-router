create table if not exists model_catalog_metadata (
  model text primary key,
  description text not null default '',
  input_modalities text[] not null default '{}',
  output_modalities text[] not null default '{}',
  context_window bigint,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (context_window is null or context_window > 0)
);
