package app

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Service) listUsageLogs(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	window := newUsageRateWindow(r, time.Now())
	whereClause, args := usageLogFilters(r, window)
	argIdx := len(args) + 1

	var total int
	countQuery := `select count(*) from request_logs rl` + whereClause
	if err := s.db.QueryRow(r.Context(), countQuery, args...).Scan(&total); err != nil {
		log.Printf("count usage logs: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}

	query := `select rl.request_id,coalesce(rl.user_id::text,''),coalesce(u.name,'') as user_name,coalesce(rl.api_key_id::text,''),coalesce(ak.name,'') as key_name,coalesce(rl.channel_id::text,''),coalesce(c.name,'') as channel_name,coalesce(rl.channel_key_id::text,''),coalesce(ck.name,'') as channel_key_name,coalesce(rl.group_id::text,'') as group_id,coalesce(coalesce(g.display_name, g.name),'') as group_name,rl.model,rl.status_code,coalesce(rl.prompt_tokens,0),coalesce(rl.completion_tokens,0),coalesce(rl.total_tokens,0),coalesce(ur.cached_prompt_tokens,0),rl.duration_ms,rl.first_token_ms,coalesce(rl.error_code,''),case when rl.error_code is not null or rl.status_code>=400 then rl.error_detail else '' end,rl.client_ip,rl.user_agent,coalesce(ur.cost,0) as cost,rl.subscription_covered,rl.created_at,coalesce(ur.usage_facts,'{}'::jsonb),coalesce(ur.billing_snapshot,'{}'::jsonb) from request_logs rl left join users u on u.id=rl.user_id left join api_keys ak on ak.id=rl.api_key_id left join channels c on c.id=rl.channel_id left join channel_api_keys ck on ck.id=rl.channel_key_id left join groups g on g.id=rl.group_id left join usage_records ur on ur.request_id=rl.request_id` + whereClause + ` order by rl.created_at desc limit $` + strconv.Itoa(argIdx) + ` offset $` + strconv.Itoa(argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("list usage logs: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()

	data := []map[string]any{}
	for rows.Next() {
		var requestID, userID, userName, apiKeyID, keyName, channelID, channelName, channelKeyID, channelKeyName, groupID, groupName, model, errorCode, errorDetail, clientIP, userAgent string
		var statusCode, duration, prompt, completion, totalTokens, cached int
		var firstTokenMs *int
		var subscriptionCovered bool
		var cost, created any
		var facts, snapshot json.RawMessage
		if err := rows.Scan(&requestID, &userID, &userName, &apiKeyID, &keyName, &channelID, &channelName, &channelKeyID, &channelKeyName, &groupID, &groupName, &model, &statusCode, &prompt, &completion, &totalTokens, &cached, &duration, &firstTokenMs, &errorCode, &errorDetail, &clientIP, &userAgent, &cost, &subscriptionCovered, &created, &facts, &snapshot); err != nil {
			log.Printf("scan usage log row: %v", err)
			continue
		}
		errorDetail = adminErrorDetail(errorDetail)
		data = append(data, map[string]any{
			"request_id":           requestID,
			"user_id":              userID,
			"user_name":            userName,
			"api_key_id":           apiKeyID,
			"key_name":             keyName,
			"channel_id":           channelID,
			"channel_name":         channelName,
			"channel_key_id":       channelKeyID,
			"channel_key_name":     channelKeyName,
			"group_id":             groupID,
			"group_name":           groupName,
			"model":                model,
			"status_code":          statusCode,
			"prompt_tokens":        prompt,
			"completion_tokens":    completion,
			"total_tokens":         totalTokens,
			"cached_prompt_tokens": cached,
			"duration_ms":          duration,
			"first_token_ms":       firstTokenMs,
			"error_code":           errorCode,
			"error_detail":         errorDetail,
			"client_ip":            clientIP,
			"user_agent":           userAgent,
			"cost":                 cost,
			"subscription":         subscriptionCovered,
			"created_at":           created,
			"usage_facts":          facts,
			"billing_snapshot":     snapshot,
		})
	}
	writeJSON(w, 200, map[string]any{"data": data, "total": total, "page": page, "page_size": pageSize})
}

func usageLogFilters(r *http.Request, window usageRateWindow) (string, []any) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	args := []any{}
	argIdx := 1
	where := []string{"coalesce(rl.error_code,'') not in ('user_concurrency_limit','group_concurrency_limit','channel_concurrency_limit')"}
	if userID != "" {
		where = append(where, "rl.user_id=$"+strconv.Itoa(argIdx)+"::bigint")
		args = append(args, userID)
		argIdx++
	}
	if model != "" {
		where = append(where, "rl.model ilike $"+strconv.Itoa(argIdx)+likeEscapeClause)
		args = append(args, "%"+escapeLikePattern(model)+"%")
		argIdx++
	}
	if channelID != "" {
		where = append(where, "rl.channel_id=$"+strconv.Itoa(argIdx)+"::bigint")
		args = append(args, channelID)
		argIdx++
	}
	if groupID != "" {
		where = append(where, "rl.group_id=$"+strconv.Itoa(argIdx)+"::uuid")
		args = append(args, groupID)
		argIdx++
	}
	if statusFilter == "success" {
		where = append(where, "rl.status_code>=200 and rl.status_code<400")
	} else if statusFilter == "error" {
		where = append(where, "rl.status_code>=400")
	} else if statusFilter == "in_progress" {
		where = append(where, "false")
	}
	if requestID != "" {
		where = append(where, "rl.request_id like $"+strconv.Itoa(argIdx))
		args = append(args, requestID+"%")
		argIdx++
	}
	if window.start != nil {
		where = append(where, "rl.created_at>=$"+strconv.Itoa(argIdx))
		args = append(args, *window.start)
		argIdx++
	}
	where = append(where, "rl.created_at<=$"+strconv.Itoa(argIdx))
	args = append(args, window.end)
	return " where " + strings.Join(where, " and "), args
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

// likeEscapeClause pins the ESCAPE character used with escapeLikePattern.
// It must stay E'\\': inside a PostgreSQL E-string \' is an escaped quote, so
// E'\' leaves the literal unterminated and every filtered query fails.
const likeEscapeClause = ` escape E'\\'`

func (s *Service) usageStats(w http.ResponseWriter, r *http.Request) {
	if rawGroupBy := strings.TrimSpace(r.URL.Query().Get("group_by")); rawGroupBy != "" {
		groupBy := parseUsageStatsDimension(rawGroupBy)
		if groupBy == "" {
			writeError(w, 400, "invalid_request", "group_by must be model or channel")
			return
		}
		s.usageStatsByDimension(w, r, groupBy)
		return
	}

	window := newUsageRateWindow(r, time.Now())
	whereClause, args := usageLogFilters(r, window)
	includeBreakdown := r.URL.Query().Get("breakdown") == "1"
	bucketExpr := "null::timestamptz"
	groupBy := ""
	if includeBreakdown {
		period := strings.TrimSpace(r.URL.Query().Get("period"))
		if period != "hour" && period != "month" {
			period = "day"
		}
		args = append(args, period)
		bucketExpr = "date_trunc($" + strconv.Itoa(len(args)) + "::text,rl.created_at)"
		groupBy = " group by grouping sets ((bucket), ()) order by bucket"
	}

	query := `select ` + bucketExpr + ` as bucket,
		count(*),count(*) filter (where rl.status_code>=200 and rl.status_code<400),count(*) filter (where rl.status_code>=400),
		coalesce(sum(rl.prompt_tokens),0),coalesce(sum(ur.cached_prompt_tokens),0),coalesce(sum(rl.completion_tokens),0),coalesce(sum(rl.total_tokens),0),
		coalesce(avg(rl.duration_ms),0),avg(rl.first_token_ms),coalesce(sum(ur.cost),0),min(rl.created_at)
		from request_logs rl left join usage_records ur on ur.request_id=rl.request_id` + whereClause + groupBy
	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("usage stats aggregate: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()

	var result map[string]any
	breakdown := []map[string]any{}
	for rows.Next() {
		var bucket, first *time.Time
		var requests, successes, errors, prompt, cached, completion, total int64
		var avgDuration, cost float64
		var avgFirstToken *float64
		if err := rows.Scan(&bucket, &requests, &successes, &errors, &prompt, &cached, &completion, &total, &avgDuration, &avgFirstToken, &cost, &first); err != nil {
			log.Printf("scan usage stats: %v", err)
			writeError(w, 500, "internal_error", "query failed")
			return
		}
		stats := map[string]any{
			"prompt_tokens":        prompt,
			"cached_prompt_tokens": cached,
			"completion_tokens":    completion,
			"total_tokens":         total,
			"avg_first_token_ms":   avgFirstToken,
		}
		if bucket != nil {
			stats["period"] = bucket.Format(time.RFC3339)
			stats["requests"] = requests
			stats["cost"] = cost
			breakdown = append(breakdown, stats)
			continue
		}
		rpm, tpm := window.rates(requests, total, first)
		stats["rpm"] = rpm
		stats["tpm"] = tpm
		stats["total_requests"] = requests
		stats["success_count"] = successes
		stats["error_count"] = errors
		stats["avg_duration_ms"] = avgDuration
		stats["total_cost"] = cost
		result = stats
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate usage stats: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	if includeBreakdown {
		result["breakdown"] = breakdown
	}
	writeJSON(w, 200, result)
}
