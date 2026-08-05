package app

import (
	"context"
	"net/http"
	"strings"
)

// checkChannelQuota evaluates every matching channel quota row in one query.
// Each row's usage window is aggregated by a lateral join on request_logs.
func (s *Service) checkChannelQuota(ctx context.Context, channelID int64, model string) error {
	rows, err := s.db.Query(ctx, `select q.max_requests,q.max_tokens,agg.requests,agg.tokens
	from channel_quota_limits q
	cross join lateral (
		select count(*) as requests, coalesce(sum(rl.total_tokens),0) as tokens
		from request_logs rl
		where rl.channel_id=$1 and rl.created_at >= now() - ('1 '||q."window")::interval
	) agg
	where q.channel_id=$1 and (q.max_requests is not null or q.max_tokens is not null)`, channelID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var maxRequests, maxTokens *int64
		var count, tokens int64
		if rows.Scan(&maxRequests, &maxTokens, &count, &tokens) != nil {
			return errInvalid
		}
		if (maxRequests != nil && count >= *maxRequests) || (maxTokens != nil && tokens >= *maxTokens) {
			return errInvalid
		}
	}
	return rows.Err()
}

// channelQuotaUsage returns the current usage and limits for a channel, grouped by window.
func (s *Service) channelQuotaUsage(ctx context.Context, channelID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `select q."window",q.max_requests,q.max_tokens,
		count(rl.*) as requests, coalesce(sum(rl.total_tokens),0) as tokens
	from channel_quota_limits q
	cross join lateral (
		select * from request_logs rl
		where rl.channel_id=$1 and rl.created_at >= now() - ('1 '||q."window")::interval
	) rl
	group by q."window",q.max_requests,q.max_tokens
	order by q."window"`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var window string
		var maxRequests, maxTokens *int64
		var requests, tokens int64
		if err := rows.Scan(&window, &maxRequests, &maxTokens, &requests, &tokens); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"window":       window,
			"max_requests": maxRequests,
			"max_tokens":   maxTokens,
			"requests":     requests,
			"tokens":       tokens,
		})
	}
	return result, rows.Err()
}

// listChannelQuotaLimits returns all quota limit rows for a channel.
func (s *Service) listChannelQuotaLimits(ctx context.Context, channelID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `select id,"window",max_requests,max_tokens,created_at from channel_quota_limits where channel_id=$1 order by "window"`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, window string
		var maxRequests, maxTokens *int64
		var created any
		if err := rows.Scan(&id, &window, &maxRequests, &maxTokens, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id":           id,
			"window":       window,
			"max_requests": maxRequests,
			"max_tokens":   maxTokens,
			"created_at":   created,
		})
	}
	return result, rows.Err()
}

// getChannelQuotaHandler returns quota limits and current usage for a channel.
func (s *Service) getChannelQuotaHandler(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	usage, err := s.channelQuotaUsage(r.Context(), channelID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	limits, err := s.listChannelQuotaLimits(r.Context(), channelID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	writeJSON(w, 200, map[string]any{"limits": limits, "usage": usage})
}

// upsertChannelQuotaHandler creates or updates a channel quota limit.
func (s *Service) upsertChannelQuotaHandler(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	var in struct {
		Window      string `json:"window"`
		MaxRequests *int64 `json:"max_requests"`
		MaxTokens   *int64 `json:"max_tokens"`
	}
	if decode(r, &in) != nil || (in.Window != "minute" && in.Window != "day" && in.Window != "month") || (in.MaxRequests == nil && in.MaxTokens == nil) {
		writeError(w, 400, "invalid_request", "window and a limit are required")
		return
	}
	if !validQuotaLimit(in.MaxRequests) || !validQuotaLimit(in.MaxTokens) {
		writeError(w, 400, "invalid_request", "limits must be between 0 and 1e12")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into channel_quota_limits(id,channel_id,"window",max_requests,max_tokens) values($1,$2,$3,$4,$5) on conflict (channel_id,"window") do update set max_requests=excluded.max_requests,max_tokens=excluded.max_tokens`, id, channelID, in.Window, in.MaxRequests, in.MaxTokens)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save quota")
		return
	}
	s.audit(r, "channel.quota_updated", "channel", channelID, map[string]any{"window": in.Window, "max_requests": in.MaxRequests, "max_tokens": in.MaxTokens})
	writeJSON(w, 200, map[string]any{"id": id})
}

// deleteChannelQuotaHandler removes a channel quota limit.
func (s *Service) deleteChannelQuotaHandler(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	window := strings.TrimSpace(r.URL.Query().Get("window"))
	if window == "" {
		writeError(w, 400, "invalid_request", "window query parameter is required")
		return
	}
	_, err := s.db.Exec(r.Context(), `delete from channel_quota_limits where channel_id=$1 and "window"=$2`, channelID, window)
	if err != nil {
		writeError(w, 500, "internal_error", "could not delete quota")
		return
	}
	s.audit(r, "channel.quota_deleted", "channel", channelID, map[string]any{"window": window})
	w.WriteHeader(http.StatusNoContent)
}

// channelUsageStatsHandler returns aggregated usage statistics for a single channel.
func (s *Service) channelUsageStatsHandler(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	var totalRequests, totalPrompt, totalCompletion, totalTokens int64
	var avgDuration float64
	var totalCost float64
	err := s.db.QueryRow(r.Context(), `select count(*),coalesce(sum(rl.prompt_tokens),0),coalesce(sum(rl.completion_tokens),0),coalesce(sum(rl.total_tokens),0),coalesce(avg(rl.duration_ms),0),coalesce(sum(ur.cost),0) from request_logs rl left join usage_records ur on ur.request_id=rl.request_id where rl.channel_id=$1`, channelID).Scan(&totalRequests, &totalPrompt, &totalCompletion, &totalTokens, &avgDuration, &totalCost)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	var successCount, errorCount int64
	_ = s.db.QueryRow(r.Context(), `select coalesce(sum(case when status_code>=200 and status_code<400 then 1 else 0 end),0),coalesce(sum(case when status_code>=400 then 1 else 0 end),0) from request_logs rl where rl.channel_id=$1`, channelID).Scan(&successCount, &errorCount)
	writeJSON(w, 200, map[string]any{
		"total_requests":    totalRequests,
		"success_count":     successCount,
		"error_count":       errorCount,
		"prompt_tokens":     totalPrompt,
		"completion_tokens": totalCompletion,
		"total_tokens":      totalTokens,
		"total_cost":        totalCost,
		"avg_duration_ms":   avgDuration,
	})
}
