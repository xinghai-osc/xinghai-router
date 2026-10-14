drop trigger if exists config_invalidation_update on users;
create trigger config_invalidation_update after update on users for each row execute function notify_config_invalidation();
