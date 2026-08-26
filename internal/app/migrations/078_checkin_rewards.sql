alter table site_settings add column if not exists checkin_base_reward numeric(20,8) not null default 1 check (checkin_base_reward >= 0);
alter table site_settings add column if not exists checkin_streak_bonus numeric(20,8) not null default 0.1 check (checkin_streak_bonus >= 0);
alter table site_settings add column if not exists checkin_max_bonus_days integer not null default 7 check (checkin_max_bonus_days between 1 and 365);
