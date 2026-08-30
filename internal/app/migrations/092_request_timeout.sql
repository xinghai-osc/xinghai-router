alter table site_settings
  add column if not exists request_timeout_seconds integer not null default 90
    check (request_timeout_seconds between 1 and 3600);
