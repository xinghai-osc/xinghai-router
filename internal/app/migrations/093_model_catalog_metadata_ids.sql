alter table model_catalog_metadata
  add column if not exists id uuid;

update model_catalog_metadata
set id = gen_random_uuid()
where id is null;

alter table model_catalog_metadata
  alter column id set default gen_random_uuid(),
  alter column id set not null;

alter table model_catalog_metadata
  drop constraint if exists model_catalog_metadata_pkey;

do $$
begin
  if not exists (
    select 1 from pg_constraint
    where conrelid = 'model_catalog_metadata'::regclass
      and conname = 'model_catalog_metadata_pkey'
  ) then
    alter table model_catalog_metadata
      add constraint model_catalog_metadata_pkey primary key (id);
  end if;
end
$$;

do $$
begin
  if not exists (
    select 1 from pg_constraint
    where conrelid = 'model_catalog_metadata'::regclass
      and conname = 'model_catalog_metadata_model_key'
  ) then
    alter table model_catalog_metadata
      add constraint model_catalog_metadata_model_key unique (model);
  end if;
end
$$;
