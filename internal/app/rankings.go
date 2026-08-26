package app

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
)

type modelRanking struct {
	Rank         int     `json:"rank"`
	PreviousRank int     `json:"previous_rank,omitempty"`
	Model        string  `json:"model_name"`
	Vendor       string  `json:"vendor"`
	Tokens       int64   `json:"total_tokens"`
	Share        float64 `json:"share"`
	Growth       float64 `json:"growth_pct"`
}
type vendorRanking struct {
	Rank        int     `json:"rank"`
	Vendor      string  `json:"vendor"`
	Tokens      int64   `json:"total_tokens"`
	Share       float64 `json:"share"`
	Growth      float64 `json:"growth_pct"`
	ModelsCount int     `json:"models_count"`
	TopModel    string  `json:"top_model"`
}
type rankingMover struct {
	Model       string  `json:"model_name"`
	Vendor      string  `json:"vendor"`
	RankDelta   int     `json:"rank_delta"`
	CurrentRank int     `json:"current_rank"`
	Growth      float64 `json:"growth_pct"`
}
type rankingTotals struct {
	model             string
	current, previous int64
}
type userRanking struct {
	Rank     int     `json:"rank"`
	Name     string  `json:"name"`
	Tokens   int64   `json:"total_tokens"`
	Cost     float64 `json:"total_cost"`
	Share    float64 `json:"share"`
	Growth   float64 `json:"growth_pct"`
	Requests int64   `json:"requests"`
	TopModel string  `json:"top_model"`
}

// maskName keeps only the first visible character of a display name so the
// public leaderboard does not expose full user identities.
func maskName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) == 0 {
		return "***"
	}
	return string(runes[0]) + "***"
}

// modelVendor preserves the default labels used by older callers and tests.
func modelVendor(model string) string {
	return providerForModel(model, []modelProvider{
		{Name: "OpenAI", Prefixes: []string{"gpt-", "o1", "o3", "o4"}, Priority: 10},
		{Name: "Anthropic", Prefixes: []string{"claude"}, Priority: 20},
		{Name: "Google", Prefixes: []string{"gemini"}, Priority: 30},
		{Name: "DeepSeek", Prefixes: []string{"deepseek"}, Priority: 40},
		{Name: "Alibaba", Prefixes: []string{"qwen", "qwq"}, Priority: 50},
	}).Name
}

func rankingDuration(period string) (time.Duration, bool) {
	switch period {
	case "today":
		return 24 * time.Hour, true
	case "week":
		return 7 * 24 * time.Hour, true
	case "month":
		return 30 * 24 * time.Hour, true
	case "year":
		return 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func growthPercent(current, previous int64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	return float64(current-previous) / float64(previous) * 100
}

// rankingsPayload is the full response body for one period; it is memoised so
// bursts of public requests cannot force repeated full-table aggregations.
type rankingsPayload struct {
	Models      []modelRanking
	Vendors     []vendorRanking
	Movers      []rankingMover
	Droppers    []rankingMover
	Users       []userRanking
	TotalTokens int64
	UpdatedAt   time.Time
}

func (s *Service) rankings(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}
	if _, ok := rankingDuration(period); !ok {
		writeError(w, 400, "invalid_request", "period must be today, week, month, or year")
		return
	}
	payload, err := s.rankingsCache.get(r.Context(), period, func(_ context.Context) (rankingsPayload, error) {
		return s.computeRankings(r, period)
	})
	if err != nil {
		writeError(w, 500, "internal_error", "could not load rankings")
		return
	}
	writeJSON(w, 200, map[string]any{"period": period, "models": payload.Models, "vendors": payload.Vendors, "top_movers": payload.Movers, "top_droppers": payload.Droppers, "users": payload.Users, "total_tokens": payload.TotalTokens, "updated_at": payload.UpdatedAt})
}

func (s *Service) computeRankings(r *http.Request, period string) (rankingsPayload, error) {
	duration, _ := rankingDuration(period)
	now := time.Now().UTC()
	providers := s.providers(r)
	start := now.Add(-duration)
	previousStart := start.Add(-duration)
	rows, err := s.db.Query(r.Context(), `select rl.model,coalesce(sum(rl.prompt_tokens+rl.completion_tokens) filter(where rl.created_at >= $1),0),coalesce(sum(rl.prompt_tokens+rl.completion_tokens) filter(where rl.created_at < $1),0) from request_logs rl where rl.created_at >= $2 and rl.created_at < $3 and rl.status_code >= 200 and rl.status_code < 400 group by rl.model`, start, previousStart, now)
	if err != nil {
		return rankingsPayload{}, err
	}
	defer rows.Close()
	totals := []rankingTotals{}
	for rows.Next() {
		var item rankingTotals
		if rows.Scan(&item.model, &item.current, &item.previous) == nil {
			totals = append(totals, item)
		}
	}
	if rows.Err() != nil {
		return rankingsPayload{}, rows.Err()
	}
	previous := append([]rankingTotals(nil), totals...)
	sort.Slice(previous, func(i, j int) bool { return previous[i].previous > previous[j].previous })
	previousRanks := map[string]int{}
	rank := 0
	for _, item := range previous {
		if item.previous > 0 {
			rank++
			previousRanks[item.model] = rank
		}
	}
	sort.Slice(totals, func(i, j int) bool { return totals[i].current > totals[j].current })
	var allTokens int64
	for _, item := range totals {
		allTokens += item.current
	}
	models := []modelRanking{}
	for _, item := range totals {
		if item.current <= 0 || len(models) == 20 {
			continue
		}
		share := 0.0
		if allTokens > 0 {
			share = float64(item.current) / float64(allTokens)
		}
		models = append(models, modelRanking{len(models) + 1, previousRanks[item.model], item.model, providerForModel(item.model, providers).Name, item.current, share, growthPercent(item.current, item.previous)})
	}
	type vendorTotals struct {
		current, previous int64
		models            map[string]int64
	}
	byVendor := map[string]*vendorTotals{}
	for _, item := range totals {
		vendor := providerForModel(item.model, providers).Name
		if byVendor[vendor] == nil {
			byVendor[vendor] = &vendorTotals{models: map[string]int64{}}
		}
		byVendor[vendor].current += item.current
		byVendor[vendor].previous += item.previous
		if item.current > 0 {
			byVendor[vendor].models[item.model] = item.current
		}
	}
	vendors := []vendorRanking{}
	for name, item := range byVendor {
		if item.current <= 0 {
			continue
		}
		top := ""
		var topTokens int64
		for model, tokens := range item.models {
			if tokens > topTokens {
				top, topTokens = model, tokens
			}
		}
		vendors = append(vendors, vendorRanking{Vendor: name, Tokens: item.current, Share: float64(item.current) / float64(allTokens), Growth: growthPercent(item.current, item.previous), ModelsCount: len(item.models), TopModel: top})
	}
	sort.Slice(vendors, func(i, j int) bool { return vendors[i].Tokens > vendors[j].Tokens })
	for i := range vendors {
		vendors[i].Rank = i + 1
	}
	movers, droppers := []rankingMover{}, []rankingMover{}
	for _, item := range models {
		if item.PreviousRank == 0 {
			continue
		}
		delta := item.PreviousRank - item.Rank
		mover := rankingMover{item.Model, item.Vendor, delta, item.Rank, item.Growth}
		if delta > 0 {
			movers = append(movers, mover)
		} else if delta < 0 {
			droppers = append(droppers, mover)
		}
	}
	sort.Slice(movers, func(i, j int) bool { return movers[i].RankDelta > movers[j].RankDelta })
	sort.Slice(droppers, func(i, j int) bool { return droppers[i].RankDelta < droppers[j].RankDelta })
	if len(movers) > 6 {
		movers = movers[:6]
	}
	if len(droppers) > 6 {
		droppers = droppers[:6]
	}
	users := s.userLeaderboard(r, start, previousStart, now, allTokens)
	return rankingsPayload{Models: models, Vendors: vendors, Movers: movers, Droppers: droppers, Users: users, TotalTokens: allTokens, UpdatedAt: now}, nil
}

// userLeaderboard ranks users by token consumption within the current period.
// It reads successful requests from request_logs (including subscription-covered
// requests, which never get a usage_record) and joins usage_records only for the
// billed cost. Display names are masked before they leave the server.
func (s *Service) userLeaderboard(r *http.Request, start, previousStart, now time.Time, allTokens int64) []userRanking {
	rows, err := s.db.Query(r.Context(), `select g.user_id::text, u.name, u.leaderboard_mask_name, g.model,
		g.current, g.previous, g.cost, g.requests
		from (
			select rl.user_id, rl.model,
				coalesce(sum(rl.prompt_tokens+rl.completion_tokens) filter(where rl.created_at >= $1),0) as current,
				coalesce(sum(rl.prompt_tokens+rl.completion_tokens) filter(where rl.created_at < $1),0) as previous,
				coalesce(sum(ur.cost) filter(where rl.created_at >= $1),0)::float8 as cost,
				count(*) filter(where rl.created_at >= $1) as requests
			from request_logs rl
			left join usage_records ur on ur.request_id = rl.request_id
			where rl.created_at >= $2 and rl.created_at < $3 and rl.status_code >= 200 and rl.status_code < 400
			group by rl.user_id, rl.model
		) g join users u on u.id = g.user_id
		where u.leaderboard_opt_in`, start, previousStart, now)
	if err != nil {
		return []userRanking{}
	}
	defer rows.Close()
	type userTotals struct {
		name              string
		mask              bool
		current, previous int64
		cost              float64
		requests          int64
		modelTokens       map[string]int64
	}
	byUser := map[string]*userTotals{}
	for rows.Next() {
		var userID, name, model string
		var mask bool
		var current, previous, requests int64
		var cost float64
		if rows.Scan(&userID, &name, &mask, &model, &current, &previous, &cost, &requests) != nil {
			continue
		}
		entry := byUser[userID]
		if entry == nil {
			entry = &userTotals{name: name, mask: mask, modelTokens: map[string]int64{}}
			byUser[userID] = entry
		}
		entry.current += current
		entry.previous += previous
		entry.cost += cost
		entry.requests += requests
		if current > 0 {
			entry.modelTokens[model] += current
		}
	}
	users := []userRanking{}
	for _, entry := range byUser {
		if entry.current <= 0 {
			continue
		}
		top, topTokens := "", int64(0)
		for model, tokens := range entry.modelTokens {
			if tokens > topTokens {
				top, topTokens = model, tokens
			}
		}
		share := 0.0
		if allTokens > 0 {
			share = float64(entry.current) / float64(allTokens)
		}
		name := entry.name
		if entry.mask {
			name = maskName(name)
		}
		users = append(users, userRanking{Name: name, Tokens: entry.current, Cost: entry.cost, Share: share, Growth: growthPercent(entry.current, entry.previous), Requests: entry.requests, TopModel: top})
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Tokens > users[j].Tokens })
	if len(users) > 20 {
		users = users[:20]
	}
	for i := range users {
		users[i].Rank = i + 1
	}
	return users
}
