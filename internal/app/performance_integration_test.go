package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func performanceIntegrationService(t *testing.T) *Service {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("PERFORMANCE_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("PERFORMANCE_TEST_DATABASE_URL or TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	id, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "performance_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "create schema "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "drop schema "+quoted+" cascade"); err != nil {
			t.Error(err)
		}
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return &Service{db: db, performanceCache: newTTLCache[string, modelPerformancePayload](time.Minute)}
}

func TestModelPerformanceIntegrationBoundariesAndVisibility(t *testing.T) {
	s := performanceIntegrationService(t)
	ctx := context.Background()
	now := time.Date(2026, time.October, 19, 16, 37, 42, 123456000, time.UTC)
	start := now.Add(-24 * time.Hour)
	var publicGroup, privateGroup, userID string
	if err := s.db.QueryRow(ctx, `insert into groups(id,name,display_name,"public") values(gen_random_uuid(),'performance-public','Public',true) returning id`).Scan(&publicGroup); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `insert into groups(id,name,"public") values(gen_random_uuid(),'performance-private',false) returning id`).Scan(&privateGroup); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `insert into users(email,name) values('performance@example.test','Performance') returning id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	channels := make(map[string]string)
	for _, name := range []string{"public", "ungrouped", "private", "owned", "disabled", "auto-disabled"} {
		var channelID string
		var owner any
		if name == "owned" {
			owner = userID
		}
		if err := s.db.QueryRow(ctx, `insert into channels(name,base_url,api_key,models,enabled,auto_disabled,user_id)
			values($1,'https://example.test','fake-key','["test-model"]',$2,$3,$4) returning id`, name, name != "disabled", name == "auto-disabled", owner).Scan(&channelID); err != nil {
			t.Fatal(err)
		}
		channels[name] = channelID
		if name == "ungrouped" {
			continue
		}
		groupID := publicGroup
		if name == "private" {
			groupID = privateGroup
		}
		if _, err := s.db.Exec(ctx, `insert into channel_groups(channel_id,group_id) values($1,$2)`, channelID, groupID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(ctx, `insert into channel_groups(channel_id,group_id) values($1,$2)`, channels["public"], privateGroup); err != nil {
		t.Fatal(err)
	}
	insertLog := func(channelID, groupID, model string, at time.Time, status, latency int, firstToken any) {
		t.Helper()
		requestID, err := randomID()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(ctx, `insert into request_logs(id,request_id,channel_id,group_id,model,created_at,status_code,duration_ms,first_token_ms)
			values(gen_random_uuid(),$1,nullif($2,'')::bigint,nullif($3,'')::uuid,$4,$5,$6,$7,$8)`, requestID, channelID, groupID, model, at, status, latency, firstToken); err != nil {
			t.Fatal(err)
		}
	}
	insertLog(channels["public"], publicGroup, " test-model ", start, 200, 100, 10)
	insertLog(channels["public"], publicGroup, "test-model", start.Add(time.Hour-time.Microsecond), 500, 300, nil)
	insertLog(channels["public"], publicGroup, "test-model", start.Add(time.Hour), 302, 900, 30)
	insertLog(channels["public"], publicGroup, "test-model", start.Add(23*time.Hour), 399, 200, nil)
	insertLog(channels["public"], publicGroup, "test-model", now.Add(-time.Microsecond), 400, 500, 80)
	insertLog(channels["ungrouped"], "", "test-model", start.Add(2*time.Hour), 500, 0, 0)
	insertLog(channels["public"], publicGroup, "test-model", start.Add(-time.Microsecond), 200, 9999, 9999)
	insertLog(channels["public"], publicGroup, "test-model", now, 200, 9999, 9999)
	insertLog(channels["public"], publicGroup, "test-model", now.Add(time.Hour), 200, 9999, 9999)
	insertLog(channels["public"], publicGroup, "another-model", start, 200, 9999, 9999)
	insertLog(channels["public"], privateGroup, "test-model", start, 200, 9999, 9999)
	insertLog("", publicGroup, "test-model", start, 200, 9999, 9999)
	for _, name := range []string{"private", "owned", "disabled", "auto-disabled"} {
		insertLog(channels[name], publicGroup, "test-model", start, 200, 9999, 9999)
		insertLog(channels[name], "", "test-model", start, 200, 9999, 9999)
	}
	payload, err := s.computeModelPerformanceAt(ctx, "test-model", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Groups) != 2 || len(payload.Series) != 2 || !payload.UpdatedAt.Equal(now) || !payload.WindowStart.Equal(start) {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	for i, group := range payload.Groups {
		series := payload.Series[i]
		if series.GroupID != group.GroupID || len(series.Points) != 24 {
			t.Fatalf("unexpected series: %+v", series)
		}
		switch group.GroupID {
		case publicGroup:
			if group.Requests != 5 {
				t.Fatalf("visible requests = %d, want 5", group.Requests)
			}
			assertPerformanceNumber(t, "summary latency", group.AvgLatencyMs, 400)
			assertPerformanceOptional(t, "summary first token", group.AvgFirstTokenMs, 40)
			assertPerformanceNumber(t, "summary success", group.SuccessRate, 0.6)
			assertPerformanceNumber(t, "summary TPS", group.TPS, 5.0/86400)
			for j, point := range series.Points {
				wantRequests, wantFailures := int64(0), int64(0)
				switch j {
				case 0, 23:
					wantRequests, wantFailures = 2, 1
				case 1:
					wantRequests = 1
				}
				if point.Requests != wantRequests || point.FailedRequests != wantFailures || !point.Timestamp.Equal(start.Add(time.Duration(j)*time.Hour)) {
					t.Fatalf("bucket %d = %+v, want %d requests and %d failures", j, point, wantRequests, wantFailures)
				}
				if wantRequests == 0 && (point.AvgLatencyMs != nil || point.AvgFirstTokenMs != nil || point.SuccessRate != nil) {
					t.Fatalf("empty bucket %d has observations: %+v", j, point)
				}
			}
			assertPerformanceOptional(t, "first bucket latency", series.Points[0].AvgLatencyMs, 200)
			assertPerformanceOptional(t, "first bucket first token", series.Points[0].AvgFirstTokenMs, 10)
			assertPerformanceOptional(t, "last bucket first token", series.Points[23].AvgFirstTokenMs, 80)
		case "__public":
			if group.Requests != 1 || series.Points[2].Requests != 1 {
				t.Fatalf("unexpected ungrouped traffic: %+v", group)
			}
			assertPerformanceOptional(t, "zero first token", group.AvgFirstTokenMs, 0)
		default:
			t.Fatalf("unexpected visible group: %+v", group)
		}
	}
	accessible, err := s.modelPerformanceAccessible(ctx, "test-model")
	if err != nil || !accessible {
		t.Fatalf("public model accessible = %v, err = %v", accessible, err)
	}
	if _, err := s.db.Exec(ctx, `update channels set enabled=false where id=$1 or id=$2`, channels["public"], channels["ungrouped"]); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	s.modelPerformance(recorder, httptest.NewRequest(http.MethodGet, "/model-performance?model=test-model", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("private-only model status = %d, want 404: %s", recorder.Code, recorder.Body.String())
	}
}
