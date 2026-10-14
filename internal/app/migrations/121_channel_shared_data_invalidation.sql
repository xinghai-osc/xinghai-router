drop trigger if exists config_invalidation_update on channels;
create trigger config_invalidation_update after update on channels for each row execute function notify_config_invalidation();
