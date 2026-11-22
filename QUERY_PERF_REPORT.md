# Database query performance audit (2026-11-19)

Measured on PostgreSQL 17.12 with a synthetic production-shaped dataset:
400k request_logs, 320k usage_records, 120k wallet_ledger, 30k audit_logs,
500 api_keys, 200 users/workspaces, 20 channels. All numbers are EXPLAIN
ANALYZE execution times, warm cache, 3 runs.

## Changes shipped

### 1. checkQuota OR-rewrite (internal/app/gateway.go)

The quota-check lateral join used a CASE expression on the predicate:

    where case when q.api_key_id is null and q.user_id is not null
        then rl.user_id=$1 else rl.api_key_id=$2 end

CASE is opaque to the planner: it could only use the created_at index, and a
'total'-window quota degenerated created_at >= '-infinity' into a full-table
scan of request_logs plus a hash join of ALL usage_records — on every single
gateway request. Rewritten as an explicit OR with literal column equalities:

    where ((rl.user_id=$1 and q.api_key_id is null and q.user_id is not null)
        or (rl.api_key_id=$2 and (q.api_key_id is not null or q.user_id is null)))

The planner now builds a BitmapOr over request_logs_user_id_idx and
request_logs_api_key_idx. Verified result-equivalent across all limit shapes
(key-scoped, user-level, model-scoped, global) and covered by the existing
nullable-scope integration tests.

| scenario (4 limits incl. total window) | before | after |
|---|---|---|
| per gateway request, warm | ~297 ms | ~14 ms |
| buffers touched | 92k | 11.6k |

### 2. Migration 142_query_read_indexes.sql

    create index wallet_ledger_created_id_idx on wallet_ledger (created_at desc, id desc);
    create index audit_logs_actor_created_idx on audit_logs (actor, created_at desc);

- Admin wallet ledger page-1 (order by created_at desc, id desc) previously
  sorted all 120k rows: 18-20 ms -> 0.13 ms.
- Per-user activity logs (audit_logs filtered by actor) previously walked the
  created_at index discarding non-matching rows (3.4k scanned for 50 hits):
  4.8 ms -> 1.5 ms with window, and unbounded full-history actor search now
  uses the actor index instead of a seq scan.

## Audited and intentionally left alone (already fine)

- accountUsage / accountUsageDaily / accountUsageSummary: workspace-scoped,
  served by request_logs_workspace_idx; 0.7-5 ms.
- listUsageLogs / admin usage-stats with narrow windows: created_at index,
  sub-ms to 2 ms.
- usageStatsByDimension count query: the usage_records join is provably
  stripped by the planner (unique request_id), so no double-work fix is
  needed.
- rankings / modelPerformance / publicActivity: 30 s result caches plus
  existing composite/trimmed-model expression indexes; 0.1-6 ms per compute.
- checkQuota absent-limit fast path and channel/subscription/pricing TTL
  caches: already present and effective.
- keyQuotaUsage, accountSubscriptions, admin user list, walletLedger (user
  view), modelCatalog: all index-served, <5 ms at this volume.

## Known residual cost (documented, not changed)

Wide-window (30d+) whole-platform aggregates over request_logs x usage_records
(admin usage-stats and usage-stats?group_by=...): 107-190 ms warm because the
aggregate genuinely touches ~200k joined rows. Fixing this properly requires a
pre-aggregation/rollup table (schema + write-path change) or denormalizing
settled cost onto request_logs; both are invasive billing-path changes judged
out of scope for this pass. Covering INCLUDE indexes were prototyped and
rejected: the planner prefers seq scans at this selectivity, and the jsonb
usage_facts extraction defeats index-only scans.

## Round 2 — gateway hot-path audit follow-up (same day)

Four more changes from the gateway-path audit, each verified independently:

### 3. channelFailed moved off the retry loop (internal/app/gateway.go)

The failure bookkeeping UPDATEs (channel_api_keys / channels row, taken with a
row lock) used to run inline in the retry loop; during an upstream outage every
in-flight request serialized on the failing channel's row. channelSucceeded was
already on the background writer; channelFailed now is too. The failure_count==3
probe trigger is checked inside the task; a dropped task when the queue is
saturated only under-counts a failure that the next request re-counts.

### 4. Cluster-mode concurrency limits now cached (internal/app/concurrency_lease.go)

Cluster mode ran `select u.max_concurrency, (select g.max_concurrency ...)`
uncached on EVERY request, bypassing the user/group TTL caches that single
mode uses. New concurrencyLimitsFor helper keeps the original fail-closed
semantics (a load error still aborts with 503) while routing through
userConcurrencyCache (30s TTL, invalidated by the admin writers).

### 5. loadSubscriptionCoversModel casts fixed (internal/app/subscriptions.go)

`us.user_id::text=$1`, `us.id::text=$4`, `t.group_id::text=$3` and the
user_groups casts defeated user_subscriptions_user_tier_idx and the
user_groups PK on every subscription-cache miss. Columns are no longer cast;
parameters are typed ($1::bigint, nullif($3/$4,'')::uuid). The nullif form
preserves the empty-string semantics exactly (null uuid compares false, like
the old text cast). Row-for-row equivalence verified against the production
schema on four parameter shapes; at 7.5k user_subscriptions rows the query
went from a 1.03 ms seq scan to a 0.137 ms index scan
(user_subscriptions_user_tier_idx).

### 6. Pre-existing failures documented (NOT introduced by this work)

- migration 056_user_id_editable.sql queries information_schema.columns
  unqualified; any test that populates public.users before the isolated-schema
  tests run makes its subquery return two rows. The usage-rates /
  migrate-concurrency / performance integration tests fail on ANY DB where
  public.users already exists — reproducible with all perf changes reverted.
- TestIntegrationChannelConcurrencyAdmission seeds public channels
  (user_id NULL, no channel_groups) and calls with an ungrouped key; the
  current channel-visibility predicate (from commit 1634405, "restrict channel
  routing to the key's own group") filters them out, so the test 503s with
  model_unavailable. Query text is byte-identical at old commit d8bcdc1 and
  HEAD, and manual reproduction confirms zero rows — the test predates the
  visibility change. Fails identically with every change in this report
  reverted.
- TestIPRateLimitRedisFailureIsUnavailable / limiter policy failures: verified
  failing on a pristine HEAD module copy.
