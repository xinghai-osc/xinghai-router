-- Gateway quota checks aggregate request_logs by API key or user, optionally model.
-- Keep the timestamp range in the index so rolling-window checks avoid scanning old rows.
create index if not exists request_logs_api_key_model_created_idx
  on request_logs (api_key_id, model, created_at desc)
  include (total_tokens);

create index if not exists request_logs_user_model_created_idx
  on request_logs (user_id, model, created_at desc)
  include (total_tokens);
