-- channels.max_concurrency feeds the gateway's per-channel admission, which is
-- cached per replica, so a change to it must notify the other replicas.
-- Migration 098 built the channels update trigger from a hard-coded column list
-- that predates the column; rebuild it with max_concurrency included.
do $$
declare
  update_columns text;
  old_columns text;
  new_columns text;
begin
  select string_agg(format('%I', col), ', '),
    string_agg(format('old.%I', col), ', '),
    string_agg(format('new.%I', col), ', ')
  into update_columns, old_columns, new_columns
  from unnest(array[
    'id', 'name', 'base_url', 'api_key', 'models', 'enabled', 'priority', 'weight',
    'provider', 'auto_disabled', 'key_type', 'upstream_path', 'upstream_format',
    'request_overrides', 'ua_pool', 'user_id', 'max_concurrency'
  ]) as col;

  execute 'drop trigger if exists config_invalidation_update on channels';
  execute format(
    'create trigger config_invalidation_update after update of %s on channels for each row when ((%s) is distinct from (%s)) execute function notify_config_invalidation()',
    update_columns, old_columns, new_columns
  );
end;
$$;
