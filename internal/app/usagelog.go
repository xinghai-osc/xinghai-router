package app

import (
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

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))

	var start, end *time.Time
	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = &t
		}
	}
	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = &t
		}
	}

	args := []any{}
	argIdx := 1
	where := []string{}
	if userID != "" {
		where = append(where, "rl.user_id=$"+strconv.Itoa(argIdx))
		args = append(args, userID)
		argIdx++
	}
	if model != "" {
		where = append(where, "rl.model=$"+strconv.Itoa(argIdx))
		args = append(args, model)
		argIdx++
	}
	if channelID != "" {
		where = append(where, "rl.channel_id=$"+strconv.Itoa(argIdx))
		args = append(args, channelID)
		argIdx++
	}
	if groupID != "" {
		where = append(where, "rl.group_id=$"+strconv.Itoa(argIdx))
		args = append(args, groupID)
		argIdx++
	}
	if statusFilter == "success" {
		where = append(where, "rl.status_code>=200 and rl.status_code<400")
	} else if statusFilter == "error" {
		where = append(where, "rl.status_code>=400")
	}
	if start != nil {
		where = append(where, "rl.created_at>=$"+strconv.Itoa(argIdx))
		args = append(args, *start)
		argIdx++
	}
	if end != nil {
		where = append(where, "rl.created_at<=$"+strconv.Itoa(argIdx))
		args = append(args, *end)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " where " + strings.Join(where, " and ")
	}

	var total int
	countQuery := `select count(*) from request_logs rl` + whereClause
	if err := s.db.QueryRow(r.Context(), countQuery, args...).Scan(&total); err != nil {
		log.Printf("count usage logs: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}

	query := `select rl.request_id,rl.user_id,coalesce(u.name,'') as user_name,rl.api_key_id,rl.channel_id,coalesce(c.name,'') as channel_name,coalesce(rl.group_id::text,'') as group_id,coalesce(g.name,'') as group_name,rl.model,rl.status_code,rl.prompt_tokens,rl.completion_tokens,rl.total_tokens,rl.duration_ms,rl.error_code,coalesce(ur.cost,0) as cost,rl.created_at from request_logs rl left join users u on u.id=rl.user_id left join channels c on c.id=rl.channel_id left join groups g on g.id=rl.group_id left join usage_records ur on ur.request_id=rl.request_id` + whereClause + ` order by rl.created_at desc limit $` + strconv.Itoa(argIdx) + ` offset $` + strconv.Itoa(argIdx+1)
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
		var requestID, userID, userName, apiKeyID, channelID, channelName, groupID, groupName, model, errorCode string
		var statusCode, duration, prompt, completion, totalTokens int
		var cost, created any
		if rows.Scan(&requestID, &userID, &userName, &apiKeyID, &channelID, &channelName, &groupID, &groupName, &model, &statusCode, &prompt, &completion, &totalTokens, &duration, &errorCode, &cost, &created) != nil {
			continue
		}
		data = append(data, map[string]any{
			"request_id":       requestID,
			"user_id":          userID,
			"user_name":        userName,
			"api_key_id":       apiKeyID,
			"channel_id":       channelID,
			"channel_name":     channelName,
			"group_id":         groupID,
			"group_name":       groupName,
			"model":            model,
			"status_code":      statusCode,
			"prompt_tokens":    prompt,
			"completion_tokens": completion,
			"total_tokens":     totalTokens,
			"duration_ms":      duration,
			"error_code":       errorCode,
			"cost":             cost,
			"created_at":       created,
		})
	}
	writeJSON(w, 200, map[string]any{"data": data, "total": total, "page": page, "page_size": pageSize})
}

func (s *Service) usageStats(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))

	if period == "" {
		period = "day"
	}
	validPeriods := map[string]bool{"hour": true, "day": true, "month": true}
	if !validPeriods[period] {
		period = "day"
	}

	var start, end *time.Time
	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = &t
		}
	}
	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = &t
		}
	}

	args := []any{}
	argIdx := 1
	where := []string{}
	if userID != "" {
		where = append(where, "rl.user_id=$"+strconv.Itoa(argIdx))
		args = append(args, userID)
		argIdx++
	}
	if model != "" {
		where = append(where, "rl.model=$"+strconv.Itoa(argIdx))
		args = append(args, model)
		argIdx++
	}
	if start != nil {
		where = append(where, "rl.created_at>=$"+strconv.Itoa(argIdx))
		args = append(args, *start)
		argIdx++
	}
	if end != nil {
		where = append(where, "rl.created_at<=$"+strconv.Itoa(argIdx))
		args = append(args, *end)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " where " + strings.Join(where, " and ")
	}

	trunc := map[string]string{"hour": "date_trunc('hour',rl.created_at)", "day": "date_trunc('day',rl.created_at)", "month": "date_trunc('month',rl.created_at)"}

	aggQuery := `select count(*),coalesce(sum(rl.prompt_tokens),0),coalesce(sum(rl.completion_tokens),0),coalesce(sum(rl.total_tokens),0),coalesce(avg(rl.duration_ms),0),coalesce(sum(ur.cost),0) from request_logs rl left join usage_records ur on ur.request_id=rl.request_id` + whereClause

	var totalRequests, totalPrompt, totalCompletion, totalTokens int64
	var avgDuration float64
	var totalCost float64
	if err := s.db.QueryRow(r.Context(), aggQuery, args...).Scan(&totalRequests, &totalPrompt, &totalCompletion, &totalTokens, &avgDuration, &totalCost); err != nil {
		log.Printf("usage stats aggregate: %v", err)
		writeError(w, 500, "internal_error", "query failed")
		return
	}

	var successCount, errorCount int64
	if err := s.db.QueryRow(r.Context(), `select coalesce(sum(case when status_code>=200 and status_code<400 then 1 else 0 end),0),coalesce(sum(case when status_code>=400 then 1 else 0 end),0) from request_logs rl`+whereClause, args...).Scan(&successCount, &errorCount); err != nil {
		successCount = 0
		errorCount = 0
	}

	result := map[string]any{
		"total_requests":    totalRequests,
		"success_count":     successCount,
		"error_count":       errorCount,
		"prompt_tokens":     totalPrompt,
		"completion_tokens": totalCompletion,
		"total_tokens":      totalTokens,
		"total_cost":        totalCost,
		"avg_duration_ms":   avgDuration,
	}

	if r.URL.Query().Get("breakdown") == "1" {
		byPeriodQuery := `select ` + trunc[period] + ` as bucket,count(*),coalesce(sum(rl.prompt_tokens),0),coalesce(sum(rl.completion_tokens),0),coalesce(sum(rl.total_tokens),0),coalesce(sum(ur.cost),0) from request_logs rl left join usage_records ur on ur.request_id=rl.request_id` + whereClause + ` group by bucket order by bucket`
		rows, err := s.db.Query(r.Context(), byPeriodQuery, args...)
		if err != nil {
			log.Printf("usage stats breakdown: %v", err)
		} else {
			defer rows.Close()
			breakdown := []map[string]any{}
			for rows.Next() {
				var bucket time.Time
				var count, prompt, completion, total int64
				var cost float64
				if rows.Scan(&bucket, &count, &prompt, &completion, &total, &cost) == nil {
					breakdown = append(breakdown, map[string]any{
						"period":           bucket.Format(time.RFC3339),
						"requests":         count,
						"prompt_tokens":    prompt,
						"completion_tokens": completion,
						"total_tokens":     total,
						"cost":             cost,
					})
				}
			}
			result["breakdown"] = breakdown
		}
	}

	writeJSON(w, 200, result)
}