-- Query performance: ordering and filter indexes for ledger and audit reads.
-- The admin wallet ledger lists and sorts by (created_at desc, id desc) without a
-- supporting index, forcing a full sort over every matching row on each page.
-- Audit activity logs filter by actor for non-admin callers; the previous plan
-- walked the created_at index and dropped non-matching rows.
create index if not exists wallet_ledger_created_id_idx on wallet_ledger (created_at desc, id desc);
create index if not exists audit_logs_actor_created_idx on audit_logs (actor, created_at desc);
