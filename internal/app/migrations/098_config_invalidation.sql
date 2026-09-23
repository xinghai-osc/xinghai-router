create or replace function notify_config_invalidation() returns trigger
language plpgsql as $$
begin
  perform pg_notify('xinghai_config_invalidation', '');
  return null;
end;
$$;

do $$
declare
  table_name text;
  config record;
  update_columns text;
  old_columns text;
  new_columns text;
begin
  foreach table_name in array array[
    'pricing_rules', 'pricing_tiers', 'pricing_time_rules', 'exchange_rates',
    'groups', 'channel_groups', 'user_groups', 'model_routes', 'model_providers',
    'site_settings', 'content_policy_rules', 'quota_limits', 'channel_quota_limits',
    'subscription_plans', 'subscription_plan_model_quotas'
  ] loop
    execute format('drop trigger if exists config_invalidation on %I', table_name);
    execute format(
      'create trigger config_invalidation after insert or update or delete or truncate on %I for each statement execute function notify_config_invalidation()',
      table_name
    );
  end loop;

  for config in select * from (values
    ('channels', array[
      'id', 'name', 'base_url', 'api_key', 'models', 'enabled', 'priority', 'weight',
      'provider', 'auto_disabled', 'key_type', 'upstream_path', 'upstream_format',
      'request_overrides', 'ua_pool', 'user_id'
    ]),
    ('channel_api_keys', array['id', 'channel_id', 'key_encrypted', 'enabled', 'priority', 'created_at']),
    ('users', array['id', 'name', 'enabled', 'max_concurrency', 'leaderboard_opt_in', 'leaderboard_mask_name']),
    ('user_subscriptions', array[
      'id', 'user_id', 'plan_id', 'status', 'current_period_start', 'current_period_end',
      'auto_renew', 'cancelled_at'
    ])
  ) as configs(table_name, columns) loop
    select string_agg(format('%I', col), ', '),
      string_agg(format('old.%I', col), ', '),
      string_agg(format('new.%I', col), ', ')
    into update_columns, old_columns, new_columns
    from unnest(config.columns) as col;

    execute format('drop trigger if exists config_invalidation_insert_delete on %I', config.table_name);
    execute format(
      'create trigger config_invalidation_insert_delete after insert or delete on %I for each row execute function notify_config_invalidation()',
      config.table_name
    );
    execute format('drop trigger if exists config_invalidation_update on %I', config.table_name);
    execute format(
      'create trigger config_invalidation_update after update of %s on %I for each row when ((%s) is distinct from (%s)) execute function notify_config_invalidation()',
      update_columns, config.table_name, old_columns, new_columns
    );
    execute format('drop trigger if exists config_invalidation_truncate on %I', config.table_name);
    execute format(
      'create trigger config_invalidation_truncate after truncate on %I for each statement execute function notify_config_invalidation()',
      config.table_name
    );
  end loop;
end;
$$;
