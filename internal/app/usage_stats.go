package app

import (
	"log"
	"net/http"
	"strings"
	"time"
)

func (s *Service) usageStatsByDimension(w http.ResponseWriter, r *http.Request, groupBy string) {
	page, pageSize, offset := listPage(r)

	start, end := usageStatsRange(r)
	where := "coalesce(rl.error_code,'') not in ('user_concurrency_limit','group_concurrency_limit','channel_concurrency_limit') and rl.created_at >= $1 and rl.created_at <= $2"
	args := []any{start, end}

	keyExpr := "coalesce(nullif(trim(rl.model),''),'')"
	nameExpr := keyExpr
	join := ""
	if groupBy == "channel" {
		keyExpr = "coalesce(rl.channel_id::text,'')"
		nameExpr = "coalesce(nullif(trim(c.name),''),'')"
		join = " left join channels c on c.id=rl.channel_id"
	}

	cacheReadExpr := "coalesce(nullif(ur.usage_facts->>'cache_read_tokens','')::bigint,ur.cached_prompt_tokens,0)"
	promptExpr := "coalesce(ur.prompt_tokens,rl.prompt_tokens,0)::bigint"
	completionExpr := "coalesce(ur.completion_tokens,rl.completion_tokens,0)::bigint"
	uncachedInputExpr := "coalesce(nullif(ur.usage_facts->>'input_tokens','')::bigint,greatest(" + promptExpr + "-" + cacheReadExpr + ",0))"
	cacheWriteExpr := "coalesce(nullif(ur.usage_facts->>'cache_write_tokens','')::bigint,coalesce(nullif(ur.usage_facts->>'cache_write_5m_tokens','')::bigint,0)+coalesce(nullif(ur.usage_facts->>'cache_write_1h_tokens','')::bigint,0))"

	withQuery := `with usage as (
		select ` + keyExpr + ` as dimension_key,
			` + nameExpr + ` as dimension_name,
			` + promptExpr + ` as prompt_tokens,
			` + completionExpr + ` as completion_tokens,
			` + cacheReadExpr + ` as cache_hit_tokens,
			` + uncachedInputExpr + ` as uncached_input_tokens,
			` + cacheWriteExpr + ` as cache_write_tokens,
			coalesce(ur.cost,0)::double precision as cost
		from request_logs rl
		left join usage_records ur on ur.request_id=rl.request_id` + join + `
		where ` + where + `
	), grouped as (
		select dimension_key,
			max(dimension_name) as dimension_name,
			sum(prompt_tokens+completion_tokens)::bigint as total_tokens,
			sum(prompt_tokens)::bigint as prompt_tokens,
			sum(completion_tokens)::bigint as completion_tokens,
			sum(cache_hit_tokens)::bigint as cache_hit_tokens,
		case when sum(cache_hit_tokens+uncached_input_tokens)>0
			then round(sum(cache_hit_tokens)::numeric*100/sum(cache_hit_tokens+uncached_input_tokens),2)::double precision
			else 0::double precision end as cache_hit_rate,
			sum(uncached_input_tokens)::bigint as uncached_input_tokens,
			sum(cache_write_tokens)::bigint as cache_write_tokens,
			count(*)::bigint as requests,
			coalesce(sum(cost),0)::double precision as cost
		from usage
		group by dimension_key
	)`

	var total int
	if err := s.db.QueryRow(r.Context(), withQuery+` select count(*) from grouped`, args...).Scan(&total); err != nil {
		log.Printf("count usage stats: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}

	query := withQuery + ` select dimension_key,dimension_name,total_tokens,prompt_tokens,completion_tokens,cache_hit_tokens,cache_hit_rate,uncached_input_tokens,cache_write_tokens,requests,cost
		from grouped order by total_tokens desc,dimension_name asc,dimension_key asc limit $3 offset $4`
	args = append(args, pageSize, offset)
	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("list usage stats: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()

	data := []map[string]any{}
	for rows.Next() {
		var key, name string
		var totalTokens, promptTokens, completionTokens, cacheHitTokens, uncachedInputTokens, cacheWriteTokens, requests int64
		var cacheHitRate, cost float64
		if err := rows.Scan(&key, &name, &totalTokens, &promptTokens, &completionTokens, &cacheHitTokens, &cacheHitRate, &uncachedInputTokens, &cacheWriteTokens, &requests, &cost); err != nil {
			log.Printf("scan usage stats row: %v", err)
			continue
		}
		data = append(data, map[string]any{
			"key":                   key,
			"name":                  name,
			"total_tokens":          totalTokens,
			"prompt_tokens":         promptTokens,
			"completion_tokens":     completionTokens,
			"cache_hit_tokens":      cacheHitTokens,
			"cache_hit_rate":        cacheHitRate,
			"uncached_input_tokens": uncachedInputTokens,
			"cache_write_tokens":    cacheWriteTokens,
			"requests":              requests,
			"cost":                  cost,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate usage stats rows: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}

	writePaged(w, data, total, page, pageSize)
}

func usageStatsRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	end := now
	if value := strings.TrimSpace(r.URL.Query().Get("start")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			start = parsed
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("end")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			end = parsed
		}
	}
	if end.Before(start) {
		start, end = end, start
	}
	return start, end
}

func parseUsageStatsDimension(value string) string {
	value = strings.TrimSpace(value)
	if value == "model" || value == "channel" {
		return value
	}
	return ""
}
