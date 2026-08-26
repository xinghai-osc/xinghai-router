alter table site_settings
    add column if not exists registration_email_whitelist_enabled boolean not null default false,
    add column if not exists registration_email_whitelist text[] not null default '{}';
