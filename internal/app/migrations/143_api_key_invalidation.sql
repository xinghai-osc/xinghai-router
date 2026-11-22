-- Cluster deployments cache the API-key context per process (bounded TTL);
-- a key revoked on one replica must not stay usable on another for the whole
-- TTL. The statement trigger extends the existing config-invalidation NOTIFY
-- to api_keys writes so every replica drops its cached key contexts at once.
drop trigger if exists config_invalidation_insert_delete on api_keys;
create trigger config_invalidation_insert_delete after insert or delete on api_keys
  for each row execute function notify_config_invalidation();

drop trigger if exists config_invalidation_update on api_keys;
create trigger config_invalidation_update after update of
  id, user_id, name, secret_hash, expires_at, revoked_at, group_id, workspace_id, subscription_id, allowed_models
  on api_keys
  for each row when ((old.id, old.user_id, old.name, old.secret_hash, old.expires_at, old.revoked_at, old.group_id, old.workspace_id, old.subscription_id, old.allowed_models)
    is distinct from
    (new.id, new.user_id, new.name, new.secret_hash, new.expires_at, new.revoked_at, new.group_id, new.workspace_id, new.subscription_id, new.allowed_models))
  execute function notify_config_invalidation();

drop trigger if exists config_invalidation_truncate on api_keys;
create trigger config_invalidation_truncate after truncate on api_keys
  for each statement execute function notify_config_invalidation();
