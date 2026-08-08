-- The paginated admin channel list aggregates per-channel request_logs for
-- the rows on the current page; this index turns those per-channel scans into
-- index range scans instead of a full-table sequential scan.
create index if not exists request_logs_channel_id_idx on request_logs(channel_id, created_at desc);