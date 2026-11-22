-- The admin channel list aggregates request_logs per channel on every page load
-- (avg duration, avg first token, request count, token sum). The existing
-- (channel_id, created_at desc) index found those rows but still fetched each one
-- from the heap to read the aggregated columns, which dominated the endpoint at
-- scale. Covering the aggregated columns turns every per-channel aggregate into
-- an index-only scan with no heap fetches, and the leading columns are unchanged,
-- so it also serves the other channel-scoped reads (channel quota and usage
-- statistics) that used request_logs_channel_id_idx.
create index if not exists request_logs_channel_usage_idx
  on request_logs (channel_id, created_at desc)
  include (duration_ms, first_token_ms, total_tokens);

drop index if exists request_logs_channel_id_idx;
