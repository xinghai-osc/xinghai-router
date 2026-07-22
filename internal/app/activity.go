package app

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func (s *Service) listActivityLogs(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	parseTime := func(name string) (*time.Time, bool) {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			return nil, true
		}
		parsed, err := time.Parse(time.RFC3339, value)
		return &parsed, err == nil
	}
	start, startOK := parseTime("start")
	end, endOK := parseTime("end")
	logType := strings.TrimSpace(r.URL.Query().Get("type"))
	validTypes := map[string]bool{"": true, "request": true, "login": true, "register": true, "logout": true, "topup": true, "operation": true}
	if !startOK || !endOK || !validTypes[logType] {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid time or log type filter")
		return
	}
	if start != nil && end != nil && start.After(*end) {
		writeError(w, http.StatusBadRequest, "invalid_request", "start must not be later than end")
		return
	}

	canReadRequests := account.role == "admin" || account.permissions["logs.read"]
	canReadAudit := account.role == "admin" || account.permissions["audit.read"]
	rows, err := s.db.Query(r.Context(), `
		select id,log_type,action,user_id,user_name,model,group_id,group_name,status_code,duration_ms,prompt_tokens,completion_tokens,total_tokens,cost,details,created_at
		from (
			select rl.request_id as id,'request'::text as log_type,'model.request'::text as action,rl.user_id::text as user_id,coalesce(u.name,'已删除用户') as user_name,rl.model,coalesce(rl.group_id::text,'') as group_id,coalesce(g.name,'') as group_name,rl.status_code,rl.duration_ms,coalesce(rl.prompt_tokens,0) as prompt_tokens,coalesce(rl.completion_tokens,0) as completion_tokens,coalesce(rl.total_tokens,0) as total_tokens,coalesce(ur.cost,0) as cost,jsonb_build_object('request_id',rl.request_id,'api_key_id',rl.api_key_id,'channel_id',rl.channel_id,'error_code',rl.error_code) as details,rl.created_at
			from request_logs rl
			left join users u on u.id=rl.user_id
			left join groups g on g.id=rl.group_id
			left join usage_records ur on ur.request_id=rl.request_id
			where ($7 or rl.user_id=$6::bigint)
			union all
			select al.id::text,
				case when al.action='account.logged_in' then 'login' when al.action='account.registered' then 'register' when al.action='account.logged_out' then 'logout' when al.action='wallet.adjusted' and coalesce((al.details->>'amount')::numeric,0)>0 then 'topup' else 'operation' end,
				al.action,al.actor,coalesce(u.name,'已删除用户'),''::text,''::text,''::text,null::integer,null::integer,0,0,0,0,al.details,al.created_at
			from audit_logs al
			left join users u on u.id::text=al.actor
			where ($8 or al.actor=$6::text)
		) activity
		where ($1='' or user_id=$1)
			and ($2='' or model=$2)
			and ($3='' or group_id=$3)
			and ($4='' or log_type=$4)
			and ($5::timestamptz is null or created_at >= $5)
			and ($9::timestamptz is null or created_at <= $9)
		order by created_at desc limit 500`, strings.TrimSpace(r.URL.Query().Get("user_id")), strings.TrimSpace(r.URL.Query().Get("model")), strings.TrimSpace(r.URL.Query().Get("group_id")), logType, start, account.userID, canReadRequests, canReadAudit, end)
	if err != nil {
		log.Printf("list activity logs: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()

	data := []map[string]any{}
	for rows.Next() {
		var id, logType, action, userID, userName, model, groupID, groupName string
		var status, duration any
		var prompt, completion, total int
		var cost, created any
		var details []byte
		if rows.Scan(&id, &logType, &action, &userID, &userName, &model, &groupID, &groupName, &status, &duration, &prompt, &completion, &total, &cost, &details, &created) != nil {
			continue
		}
		data = append(data, map[string]any{"id": id, "type": logType, "action": action, "user_id": userID, "user_name": userName, "model": model, "group_id": groupID, "group_name": groupName, "status_code": status, "duration_ms": duration, "prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total, "cost": cost, "details": json.RawMessage(details), "created_at": created})
	}
	if rows.Err() != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
