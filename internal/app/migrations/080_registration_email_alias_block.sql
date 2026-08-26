alter table site_settings
    add column if not exists registration_email_alias_blocked boolean not null default false;
