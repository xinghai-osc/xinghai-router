package app

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func (s *Service) usageStatsByDimension(w http.ResponseWriter, r *http.Request, groupBy string) {
	page, pageSize, offset := listPage(r)

	start, end := usageStatsRange(r)
	where := "coalesce(rl.error_code,'') not in ('user_concurrency_limit','group_concurrency_limit','channel_concurrency_limit') and rl.created_at >= $1 and rl.created_at <= $2"

	dimension := "coalesce(trim(rl.model),'')"
	key, name := "g.dimension_key", "g.dimension_key"
	join := ""
	if groupBy == "channel" {
		dimension = "rl.channel_id"
		key, name = "coalesce(g.dimension_key::text,'')", "coalesce(trim(c.name),'')"
		join = " left join channels c on c.id=g.dimension_key"
	}

	cacheReadExpr := "coalesce(nullif(ur.usage_facts->>'cache_read_tokens','')::bigint,ur.cached_prompt_tokens,0)"
	promptExpr := "coalesce(ur.prompt_tokens,rl.prompt_tokens,0)::bigint"
	completionExpr := "coalesce(ur.completion_tokens,rl.completion_tokens,0)::bigint"
	uncachedInputExpr := "coalesce(nullif(ur.usage_facts->>'input_tokens','')::bigint,greatest(" + promptExpr + "-" + cacheReadExpr + ",0))"
	cacheWriteExpr := "coalesce(nullif(ur.usage_facts->>'cache_write_tokens','')::bigint,coalesce(nullif(ur.usage_facts->>'cache_write_5m_tokens','')::bigint,0)+coalesce(nullif(ur.usage_facts->>'cache_write_1h_tokens','')::bigint,0))"

	query := `with usage as (
		select ` + dimension + ` as dimension_key,
			` + promptExpr + ` as prompt_tokens,
			` + completionExpr + ` as completion_tokens,
			` + cacheReadExpr + ` as cache_hit_tokens,
			` + uncachedInputExpr + ` as uncached_input_tokens,
			` + cacheWriteExpr + ` as cache_write_tokens,
			coalesce(ur.cost,0)::double precision as cost
		from request_logs rl
		left join usage_records ur on ur.request_id=rl.request_id
		where ` + where + `
	), grouped as (
		select dimension_key,
			sum(prompt_tokens+completion_tokens)::bigint as total_tokens,
			sum(prompt_tokens)::bigint as prompt_tokens,
			sum(completion_tokens)::bigint as completion_tokens,
			sum(cache_hit_tokens)::bigint as cache_hit_tokens,
			sum(uncached_input_tokens)::bigint as uncached_input_tokens,
			sum(cache_write_tokens)::bigint as cache_write_tokens,
			count(*)::bigint as requests,
			coalesce(sum(cost),0)::double precision as cost
		from usage
		group by dimension_key
	), page as (
		select ` + key + ` as key, ` + name + ` as name,
			total_tokens,prompt_tokens,completion_tokens,cache_hit_tokens,uncached_input_tokens,cache_write_tokens,requests,cost,
			case when cache_hit_tokens::numeric+uncached_input_tokens>0
				then round(cache_hit_tokens::numeric*100/(cache_hit_tokens::numeric+uncached_input_tokens),2)::double precision
				else 0::double precision end as cache_hit_rate
		from grouped g` + join + `
		order by total_tokens desc,name,key limit $3 offset $4
	)
	select (select count(*) from grouped),coalesce(json_agg(page order by total_tokens desc,name,key),'[]') from page`

	var total int
	var data json.RawMessage
	if err := s.db.QueryRow(r.Context(), query, start, end, pageSize, offset).Scan(&total, &data); err != nil {
		log.Printf("list usage stats: %v", err)
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
