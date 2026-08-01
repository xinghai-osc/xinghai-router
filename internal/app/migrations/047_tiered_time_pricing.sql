-- Tiered (stepped) pricing: different per-million rates for successive token-volume
-- bands.  A tier applies to tokens *above* its from_tokens boundary, so the first
-- row should have from_tokens = 0.  The resolver picks the highest from_tokens that
-- is <= the request's total token count and uses that row's prices.
create table if not exists pricing_tiers (
  id uuid primary key,
  model text not null references pricing_rules(model) on delete cascade,
  from_tokens bigint not null default 0 check (from_tokens >= 0),
  input_per_million numeric(20,8) not null default 0,
  cached_input_per_million numeric(20,8) not null default 0,
  output_per_million numeric(20,8) not null default 0,
  created_at timestamptz not null default now()
);
create index if not exists pricing_tiers_model_idx on pricing_tiers(model, from_tokens);

-- Time-based pricing: a full price override active during a window of the week.
-- start_minute / end_minute are minutes-since-midnight in [0,1440).
-- weekdays is a 7-char string of '0'/'1' (Mon..Sun); '1111111' means every day.
-- When one or more rules match the current time, the *last* matching rule (by
-- created_at) wins, and its prices replace the base prices from pricing_rules.
create table if not exists pricing_time_rules (
  id uuid primary key,
  model text not null references pricing_rules(model) on delete cascade,
  name text not null default '',
  start_minute int not null default 0 check (start_minute >= 0 and start_minute < 1440),
  end_minute int not null default 1440 check (end_minute > 0 and end_minute <= 1440),
  weekdays char(7) not null default '1111111',
  input_per_million numeric(20,8) not null default 0,
  cached_input_per_million numeric(20,8) not null default 0,
  output_per_million numeric(20,8) not null default 0,
  enabled boolean not null default true,
  created_at timestamptz not null default now()
);
create index if not exists pricing_time_rules_model_idx on pricing_time_rules(model, enabled);
