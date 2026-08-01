-- Channel quota limits: per-channel request/token caps over a rolling window.
-- Mirrors quota_limits but scoped to channels instead of users/api_keys.
create table if not exists channel_quota_limits (
    id uuid primary key default gen_random_uuid(),
    channel_id uuid not null references channels(id) on delete cascade,
    "window" text not null check ("window" in ('minute','day','month')),
    max_requests bigint check (max_requests is null or (max_requests >= 0 and max_requests <= 1000000000000)),
    max_tokens bigint check (max_tokens is null or (max_tokens >= 0 and max_tokens <= 1000000000000)),
    created_at timestamptz not null default now(),
    unique (channel_id, "window")
);

create index if not exists idx_channel_quota_limits_channel_id on channel_quota_limits(channel_id);
