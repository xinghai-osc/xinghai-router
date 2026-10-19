alter table site_settings
  add column if not exists request_body_timeout_seconds integer not null default 30
    check (request_body_timeout_seconds between 0 and 3600);
