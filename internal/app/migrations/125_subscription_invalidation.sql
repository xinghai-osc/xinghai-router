-- Config-invalidation triggers for the reworked subscription catalogue. 114
-- dropped the legacy subscription_plans triggers (and the user_subscriptions
-- row trigger, which named the dropped plan_id column), so this migration binds
-- the new tables and rebuilds the subscription row trigger. The whole block is
-- drop-then-create so it can be re-applied, which the integration suite checks.
do $$
declare
  table_name text;
  update_columns text;
  old_columns text;
  new_columns text;
begin
  foreach table_name in array array[
    'subscriptions', 'subscription_tiers', 'subscription_tier_prices', 'subscription_tier_model_quotas'
  ] loop
    execute format('drop trigger if exists config_invalidation on %I', table_name);
    execute format(
      'create trigger config_invalidation after insert or update or delete or truncate on %I for each statement execute function notify_config_invalidation()',
      table_name
    );
  end loop;

  select string_agg(format('%I', col), ', '),
         string_agg(format('old.%I', col), ', '),
         string_agg(format('new.%I', col), ', ')
  into update_columns, old_columns, new_columns
  from unnest(array[
    'id', 'user_id', 'tier_id', 'status', 'current_period_start', 'current_period_end',
    'auto_renew', 'cancelled_at', 'billing_period', 'price', 'currency'
  ]) as col;

  execute 'drop trigger if exists config_invalidation_insert_delete on user_subscriptions';
  execute 'create trigger config_invalidation_insert_delete after insert or delete on user_subscriptions for each row execute function notify_config_invalidation()';
  execute 'drop trigger if exists config_invalidation_update on user_subscriptions';
  execute format(
    'create trigger config_invalidation_update after update of %s on user_subscriptions for each row when ((%s) is distinct from (%s)) execute function notify_config_invalidation()',
    update_columns, old_columns, new_columns
  );
  execute 'drop trigger if exists config_invalidation_truncate on user_subscriptions';
  execute 'create trigger config_invalidation_truncate after truncate on user_subscriptions for each statement execute function notify_config_invalidation()';
end;
$$;
