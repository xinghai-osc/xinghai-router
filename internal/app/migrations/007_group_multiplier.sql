alter table groups
  add column multiplier numeric(12,6) not null default 1 check (multiplier > 0);
