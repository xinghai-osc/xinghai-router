package app

import (
	"context"
	"net/http"
	"sort"
	"time"
)

// concurrencyStatusEntry is one entity with a configured concurrency limit plus
// its live in-flight count.
type concurrencyStatusEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Limit   int    `json:"limit"`
	Current int    `json:"current"`
}

type concurrencyStatusScope struct {
	Scope   string                   `json:"scope"`
	Limit   int                      `json:"limit"`
	Current int                      `json:"current"`
	Entries []concurrencyStatusEntry `json:"entries"`
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

func (s *Service) limitedChannels(ctx context.Context) ([]concurrencyStatusEntry, error) {
	return s.limitedConcurrencyEntities(ctx, `select id::text,name,'',max_concurrency from channels where max_concurrency>0`)
}

func (s *Service) limitedUsers(ctx context.Context) ([]concurrencyStatusEntry, error) {
	return s.limitedConcurrencyEntities(ctx, `select id::text,coalesce(name,''),coalesce(email,''),max_concurrency from users where max_concurrency>0`)
}

func (s *Service) limitedGroups(ctx context.Context) ([]concurrencyStatusEntry, error) {
	return s.limitedConcurrencyEntities(ctx, `select id::text,name,'',max_concurrency from groups where max_concurrency>0`)
}

func (s *Service) limitedConcurrencyEntities(ctx context.Context, query string) ([]concurrencyStatusEntry, error) {
	rows, err := s.db.Query(ctx, query)
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

func (s *Service) concurrencyCounts(ctx context.Context, scope string, entries []concurrencyStatusEntry, shared bool) ([]int, error) {
	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = entry.ID
	}
	if len(keys) == 0 {
		return []int{}, nil
	}
	if shared {
		if s.concurrencyLeases == nil {
			return nil, errConcurrencyUnavailable
		}
		return s.concurrencyLeases.counts(ctx, scope, keys)
	}
	limiter := s.concurrencyLimiterFor(scope)
	if limiter == nil {
		return nil, errConcurrencyUnavailable
	}
	live := limiter.snapshot(keys)
	counts := make([]int, len(keys))
	for i, key := range keys {
		counts[i] = live[key]
	}
	return counts, nil
}

// concurrencyStatus reports the live in-flight count against every configured
// user, group, and channel concurrency limit. In cluster mode the counts come
// from the shared Redis leases; otherwise they are this process's own counters.
func (s *Service) concurrencyStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loaders := []struct {
		scope string
		load  func(context.Context) ([]concurrencyStatusEntry, error)
	}{
		{"channel", s.limitedChannels},
		{"user", s.limitedUsers},
		{"group", s.limitedGroups},
	}
	shared := s.cfg.DeploymentMode == "cluster"
	scopes := make([]concurrencyStatusScope, 0, len(loaders))
	for _, loader := range loaders {
		entries, err := loader.load(ctx)
		if err != nil {
			writeError(w, 500, "internal_error", "query failed")
			return
		}
		counts, err := s.concurrencyCounts(ctx, loader.scope, entries, shared)
		if err != nil {
			writeError(w, 503, "concurrency_unavailable", "could not read concurrency usage")
			return
		}
		scope := concurrencyStatusScope{Scope: loader.scope, Entries: entries}
		for i := range scope.Entries {
			scope.Entries[i].Current = counts[i]
			scope.Limit += scope.Entries[i].Limit
			scope.Current += counts[i]
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
