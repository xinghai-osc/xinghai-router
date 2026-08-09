alter table site_settings add column if not exists captcha_provider text not null default '';
alter table site_settings add column if not exists corptcha_site_id text not null default '';
alter table site_settings add column if not exists corptcha_secret_encrypted text not null default '';