package app

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const performanceWindow = 24 * time.Hour
const performanceBucket = time.Hour

type modelPerformanceGroup struct {
	GroupID         string   `json:"group_id"`
	GroupName       string   `json:"group_name"`
	Requests        int64    `json:"requests"`
	TPS             float64  `json:"tps"`
	AvgLatencyMs    float64  `json:"avg_latency_ms"`
	AvgFirstTokenMs *float64 `json:"avg_first_token_ms"`
	SuccessRate     float64  `json:"success_rate"`
}

type modelPerformancePoint struct {
	Timestamp       time.Time `json:"timestamp"`
	Requests        int64     `json:"requests"`
	FailedRequests  int64     `json:"failed_requests"`
	TPS             float64   `json:"tps"`
	AvgLatencyMs    *float64  `json:"avg_latency_ms"`
	AvgFirstTokenMs *float64  `json:"avg_first_token_ms"`
	SuccessRate     *float64  `json:"success_rate"`
}

type modelPerformanceSeries struct {
	GroupID string                  `json:"group_id"`
	Points  []modelPerformancePoint `json:"points"`
}

type modelPerformancePayload struct {
	Model         string                   `json:"model"`
	WindowHours   int                      `json:"window_hours"`
	WindowStart   time.Time                `json:"window_start"`
	BucketMinutes int                      `json:"bucket_minutes"`
	Groups        []modelPerformanceGroup  `json:"groups"`
	Series        []modelPerformanceSeries `json:"series"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

type modelPerformanceBucket struct {
	GroupID         string
	GroupName       string
	Index           int
	Requests        int64
	Success         int64
	LatencyMs       int64
	FirstTokenMs    int64
	FirstTokenCount int64
}

// modelPerformance serves public per-group performance (requests/second,
// average latency, success rate) for a single model over the last 24 hours.
// It is rate-limited like /rankings and memoised so the aggregate query is not
// re-run on every panel open.
func (s *Service) modelPerformance(w http.ResponseWriter, r *http.Request) {
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	if model == "" {
		writeError(w, 400, "invalid_request", "model is required")
		return
	}
	accessible, err := s.modelPerformanceAccessible(r.Context(), model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not verify model performance")
		return
	}
	if !accessible {
		writeError(w, http.StatusNotFound, "not_found", "model performance not found")
		return
	}
	payload, err := s.performanceCache.get(r.Context(), model, func(ctx context.Context) (modelPerformancePayload, error) {
		return s.computeModelPerformance(ctx, model)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load model performance")
		return
	}
	writeJSON(w, 200, payload)
}

func (s *Service) modelPerformanceAccessible(ctx context.Context, model string) (bool, error) {
	var accessible bool
	err := s.db.QueryRow(ctx, `select exists(
		select 1
		from channels c
		left join channel_groups cg on cg.channel_id=c.id
		left join groups g on g.id=cg.group_id
		where c.enabled and not c.auto_disabled and c.user_id is null
			and (g.id is null or g."public")
			and exists(
				select 1
				from jsonb_array_elements_text(c.models) as item(model)
				where trim(item.model)=$1
			)
		union all
		select 1
		from model_routes m
		join channels c on c.id=m.channel_id
		left join channel_groups cg on cg.channel_id=c.id
		left join groups g on g.id=cg.group_id
		where c.enabled and not c.auto_disabled and c.user_id is null
			and m.enabled and not m.hidden and trim(m.public_model)=$1
			and (g.id is null or g."public")
	)`, model).Scan(&accessible)
	return accessible, err
}

func (s *Service) computeModelPerformance(ctx context.Context, model string) (modelPerformancePayload, error) {
	return s.computeModelPerformanceAt(ctx, model, time.Now().UTC().Truncate(time.Microsecond))
}

func (s *Service) computeModelPerformanceAt(ctx context.Context, model string, now time.Time) (modelPerformancePayload, error) {
	start := now.Add(-performanceWindow)
	rows, err := s.db.Query(ctx, `
		with visible_channels as (
			select c.id
			from channels c
			where c.enabled and not c.auto_disabled and c.user_id is null
				and (
					not exists(select 1 from channel_groups cg where cg.channel_id=c.id)
					or exists(
						select 1
						from channel_groups cg
						join groups g on g.id=cg.group_id
						where cg.channel_id=c.id and g."public"
					)
				)
		), visible_groups as (
			select g.id
			from groups g
			where g."public"
		)
		select coalesce(rl.group_id::text, '__public'),
			coalesce(g.display_name, g.name, '公共'),
			floor(extract(epoch from (rl.created_at - $2::timestamptz)) / $4)::integer,
			count(*),
			count(*) filter (where rl.status_code >= 200 and rl.status_code < 400),
			sum(rl.duration_ms),
			coalesce(sum(rl.first_token_ms), 0),
			count(rl.first_token_ms)
		from request_logs rl
		join visible_channels vc on vc.id=rl.channel_id
		left join groups g on g.id = rl.group_id
		where trim(rl.model) = $1 and rl.created_at >= $2 and rl.created_at < $3
			and (rl.group_id is null or exists(select 1 from visible_groups vg where vg.id=rl.group_id))
		group by rl.group_id, g.id, g.display_name, g.name, 3
		order by coalesce(g.display_name, g.name, '公共'), rl.group_id, 3`, model, start, now, performanceBucket.Seconds())
	if err != nil {
		return modelPerformancePayload{}, err
	}
	defer rows.Close()
	buckets := []modelPerformanceBucket{}
	for rows.Next() {
		var bucket modelPerformanceBucket
		if err := rows.Scan(&bucket.GroupID, &bucket.GroupName, &bucket.Index, &bucket.Requests, &bucket.Success, &bucket.LatencyMs, &bucket.FirstTokenMs, &bucket.FirstTokenCount); err != nil {
			return modelPerformancePayload{}, err
		}
		buckets = append(buckets, bucket)
	}
	if rows.Err() != nil {
		return modelPerformancePayload{}, rows.Err()
	}
	return buildModelPerformancePayload(model, now, buckets), nil
}

func buildModelPerformancePayload(model string, now time.Time, buckets []modelPerformanceBucket) modelPerformancePayload {
	payload := modelPerformancePayload{
		Model:         model,
		WindowHours:   int(performanceWindow.Hours()),
		WindowStart:   now.Add(-performanceWindow),
		BucketMinutes: int(performanceBucket.Minutes()),
		Groups:        []modelPerformanceGroup{},
		Series:        []modelPerformanceSeries{},
		UpdatedAt:     now,
	}
	indices := make(map[string]int)
	totals := []modelPerformanceBucket{}
	for _, bucket := range buckets {
		index, ok := indices[bucket.GroupID]
		if !ok {
			index = len(payload.Groups)
			indices[bucket.GroupID] = index
			payload.Groups = append(payload.Groups, modelPerformanceGroup{GroupID: bucket.GroupID, GroupName: bucket.GroupName})
			points := make([]modelPerformancePoint, int(performanceWindow/performanceBucket))
			for i := range points {
				points[i].Timestamp = payload.WindowStart.Add(time.Duration(i) * performanceBucket)
			}
			payload.Series = append(payload.Series, modelPerformanceSeries{GroupID: bucket.GroupID, Points: points})
			totals = append(totals, modelPerformanceBucket{})
		}
		point := &payload.Series[index].Points[bucket.Index]
		point.Requests = bucket.Requests
		point.FailedRequests = bucket.Requests - bucket.Success
		point.TPS = float64(bucket.Requests) / performanceBucket.Seconds()
		point.AvgLatencyMs = performanceAverage(bucket.LatencyMs, bucket.Requests)
		point.AvgFirstTokenMs = performanceAverage(bucket.FirstTokenMs, bucket.FirstTokenCount)
		point.SuccessRate = performanceAverage(bucket.Success, bucket.Requests)
		total := &totals[index]
		total.Requests += bucket.Requests
		total.Success += bucket.Success
		total.LatencyMs += bucket.LatencyMs
		total.FirstTokenMs += bucket.FirstTokenMs
		total.FirstTokenCount += bucket.FirstTokenCount
	}
	for i, total := range totals {
		group := &payload.Groups[i]
		group.Requests = total.Requests
		group.TPS = float64(total.Requests) / performanceWindow.Seconds()
		if total.Requests > 0 {
			group.AvgLatencyMs = float64(total.LatencyMs) / float64(total.Requests)
			group.SuccessRate = float64(total.Success) / float64(total.Requests)
		}
		group.AvgFirstTokenMs = performanceAverage(total.FirstTokenMs, total.FirstTokenCount)
	}
	return payload
}

func performanceAverage(sum, count int64) *float64 {
	if count == 0 {
		return nil
	}
	average := float64(sum) / float64(count)
	return &average
}
