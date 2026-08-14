-- Allow a channel to be restricted to a single user. When channels.user_id is
-- set, only that user's API keys may route to the channel and its models are
-- only listed for them. NULL keeps the channel open to its bound groups.
alter table channels add column if not exists user_id bigint;

do $$
begin
  if exists (
    select 1 from information_schema.columns
    where table_name = 'channels' and column_name = 'user_id' and data_type = 'uuid'
  ) then
    alter table channels alter column user_id type bigint using null;
  end if;
  if not exists (select 1 from pg_constraint where conname = 'channels_user_id_fkey') then
    alter table channels add constraint channels_user_id_fkey foreign key (user_id) references users(id) on delete set null;
  end if;
end $$;

create index if not exists channels_user_id_idx on channels(user_id);
