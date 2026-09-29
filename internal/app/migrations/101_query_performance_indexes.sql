create index if not exists api_keys_user_created_idx
  on api_keys(user_id, created_at desc);

create index if not exists request_logs_model_created_idx
  on request_logs(model, created_at desc);

create index if not exists request_logs_trimmed_model_created_idx
  on request_logs ((trim(model)), created_at desc);

create index if not exists model_routes_channel_model_idx
  on model_routes(channel_id, public_model);

create index if not exists channel_api_keys_channel_enabled_order_idx
  on channel_api_keys(channel_id, enabled, priority desc, created_at);

create index if not exists quota_limits_api_key_window_idx
  on quota_limits(api_key_id, "window");
