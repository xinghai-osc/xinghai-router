package app

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// concurrencyStatusEntry is one channel, user, or group with a concurrency
// limit, or with requests currently in flight. Current is the number the limit
// is enforced against (shared across replicas in cluster mode); Local is this
// replica's own share, which is all that is knowable for a key without a limit.
type concurrencyStatusEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Limit   int    `json:"limit"`
	Current int    `json:"current"`
	Local   int    `json:"local"`
}

type concurrencyStatusScope struct {
	Scope     string                   `json:"scope"`
	Limit     int                      `json:"limit"`
	Current   int                      `json:"current"`
	Unlimited int                      `json:"unlimited"`
	Entries   []concurrencyStatusEntry `json:"entries"`
}

type concurrencySource struct {
	scope   string
	limited func(context.Context) ([]concurrencyStatusEntry, error)
	byIDs   func(context.Context, []string) ([]concurrencyStatusEntry, error)
}

func (s *Service) concurrencyLimiterFor(scope string) *GroupLimiter {
	switch scope {
	case "channel":
		return s.channelLimiter
	case "user":
		return s.userLimiter
	case "group":
		return s.groupLimiter
	}
	return nil
}

func (s *Service) queryConcurrencyEntries(ctx context.Context, query string, args ...any) ([]concurrencyStatusEntry, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []concurrencyStatusEntry{}
	for rows.Next() {
		var entry concurrencyStatusEntry
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.Email, &entry.Limit); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// intIDs converts the scope's textual keys into the bigint array the id lookup
// expects. Keys that are not numeric cannot exist in the table, so they are
// dropped and the caller reports them as id-only entries instead.
func intIDs(ids []string) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if value, err := strconv.ParseInt(id, 10, 64); err == nil {
			out = append(out, value)
		}
	}
	return out
}

func (s *Service) concurrencySources() []concurrencySource {
	return []concurrencySource{
		{
			scope: "channel",
			limited: func(ctx context.Context) ([]concurrencyStatusEntry, error) {
				return s.queryConcurrencyEntries(ctx, `select id::text,name,'',max_concurrency from channels where max_concurrency>0`)
			},
			byIDs: func(ctx context.Context, ids []string) ([]concurrencyStatusEntry, error) {
				return s.queryConcurrencyEntries(ctx, `select id::text,name,'',coalesce(max_concurrency,0) from channels where id = any($1::bigint[])`, intIDs(ids))
			},
		},
		{
			scope: "user",
			limited: func(ctx context.Context) ([]concurrencyStatusEntry, error) {
				return s.queryConcurrencyEntries(ctx, `select id::text,coalesce(name,''),coalesce(email,''),max_concurrency from users where max_concurrency>0`)
			},
			byIDs: func(ctx context.Context, ids []string) ([]concurrencyStatusEntry, error) {
				return s.queryConcurrencyEntries(ctx, `select id::text,coalesce(name,''),coalesce(email,''),coalesce(max_concurrency,0) from users where id = any($1::bigint[])`, intIDs(ids))
			},
		},
		{
			scope: "group",
			limited: func(ctx context.Context) ([]concurrencyStatusEntry, error) {
				return s.queryConcurrencyEntries(ctx, `select id::text,name,'',max_concurrency from groups where max_concurrency>0`)
			},
			byIDs: func(ctx context.Context, ids []string) ([]concurrencyStatusEntry, error) {
				return s.queryConcurrencyEntries(ctx, `select id::text,name,'',coalesce(max_concurrency,0) from groups where id::text = any($1::text[])`, ids)
			},
		},
	}
}

// concurrencyEntries lists every entity that either has a configured limit or
// currently has requests in flight, with this replica's count and the count the
// limit is enforced against.
func (s *Service) concurrencyEntries(ctx context.Context, source concurrencySource, shared bool) ([]concurrencyStatusEntry, error) {
	entries, err := source.limited(ctx)
	if err != nil {
		return nil, err
	}
	limiter := s.concurrencyLimiterFor(source.scope)
	if limiter == nil {
		return nil, errConcurrencyUnavailable
	}
	active := limiter.active()
	index := make(map[string]int, len(entries))
	for i, entry := range entries {
		index[entry.ID] = i
	}
	extraIDs := make([]string, 0, len(active))
	for id, current := range active {
		if current <= 0 {
			continue
		}
		if _, ok := index[id]; !ok {
			extraIDs = append(extraIDs, id)
		}
	}
	if len(extraIDs) > 0 {
		named, err := source.byIDs(ctx, extraIDs)
		if err != nil {
			return nil, err
		}
		for _, entry := range named {
			if _, ok := index[entry.ID]; ok {
				continue
			}
			index[entry.ID] = len(entries)
			entries = append(entries, entry)
		}
		// An entity deleted mid-request still holds a slot until it finishes.
		for _, id := range extraIDs {
			if _, ok := index[id]; ok {
				continue
			}
			index[id] = len(entries)
			entries = append(entries, concurrencyStatusEntry{ID: id})
		}
	}
	for i := range entries {
		entries[i].Local = active[entries[i].ID]
		entries[i].Current = entries[i].Local
	}
	if !shared {
		return entries, nil
	}
	limitedIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Limit > 0 {
			limitedIDs = append(limitedIDs, entry.ID)
		}
	}
	counts, err := s.concurrencyCounts(ctx, source.scope, limitedIDs, true)
	if err != nil {
		return nil, err
	}
	for i, id := range limitedIDs {
		entries[index[id]].Current = counts[i]
	}
	return entries, nil
}

func (s *Service) concurrencyCounts(ctx context.Context, scope string, ids []string, shared bool) ([]int, error) {
	if len(ids) == 0 {
		return []int{}, nil
	}
	if shared {
		if s.concurrencyLeases == nil {
			return nil, errConcurrencyUnavailable
		}
		return s.concurrencyLeases.counts(ctx, scope, ids)
	}
	limiter := s.concurrencyLimiterFor(scope)
	if limiter == nil {
		return nil, errConcurrencyUnavailable
	}
	live := limiter.snapshot(ids)
	counts := make([]int, len(ids))
	for i, id := range ids {
		counts[i] = live[id]
	}
	return counts, nil
}

// concurrencyStatus reports live concurrency for channels, users, and groups.
// Entities without a limit appear while they have requests in flight; their
// count is this replica's own, because a key that never takes a lease has
// nothing shared to read.
func (s *Service) concurrencyStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shared := s.cfg.DeploymentMode == "cluster"
	sources := s.concurrencySources()
	scopes := make([]concurrencyStatusScope, 0, len(sources))
	for _, source := range sources {
		entries, err := s.concurrencyEntries(ctx, source, shared)
		if err != nil {
			if err == errConcurrencyUnavailable {
				writeError(w, 503, "concurrency_unavailable", "could not read concurrency usage")
				return
			}
			writeError(w, 500, "internal_error", "query failed")
			return
		}
		scope := concurrencyStatusScope{Scope: source.scope, Entries: entries}
		for _, entry := range entries {
			scope.Limit += entry.Limit
			scope.Current += entry.Current
			if entry.Limit <= 0 {
				scope.Unlimited++
			}
		}
		sort.SliceStable(scope.Entries, func(i, j int) bool {
			if scope.Entries[i].Current != scope.Entries[j].Current {
				return scope.Entries[i].Current > scope.Entries[j].Current
			}
			return scope.Entries[i].Name < scope.Entries[j].Name
		})
		scopes = append(scopes, scope)
	}
	writeJSON(w, 200, map[string]any{
		"deployment_mode": s.cfg.DeploymentMode,
		"shared":          shared,
		"generated_at":    time.Now().UTC(),
		"scopes":          scopes,
	})
}
