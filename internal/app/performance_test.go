package app

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestModelPerformanceRequiresModel(t *testing.T) {
	s := &Service{}
	recorder := httptest.NewRecorder()
	s.modelPerformance(recorder, httptest.NewRequest(http.MethodGet, "/model-performance", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestModelPerformanceHourlyGapsAndWeightedSummary(t *testing.T) {
	now := time.Date(2026, time.October, 19, 16, 37, 42, 123456000, time.UTC)
	payload := buildModelPerformancePayload("test-model", now, []modelPerformanceBucket{
		{GroupID: "a", GroupName: "Shared name", Index: 0, Requests: 1, Success: 1, LatencyMs: 100, FirstTokenMs: 20, FirstTokenCount: 1},
		{GroupID: "a", GroupName: "Shared name", Index: 2, Requests: 9, Success: 3, LatencyMs: 4500, FirstTokenMs: 300, FirstTokenCount: 3},
		{GroupID: "a", GroupName: "Shared name", Index: 23, Requests: 2, Success: 2, LatencyMs: 200},
		{GroupID: "b", GroupName: "Shared name", Index: 2, Requests: 1},
	})
	if payload.Model != "test-model" || payload.WindowHours != 24 || payload.BucketMinutes != 60 || !payload.UpdatedAt.Equal(now) || !payload.WindowStart.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("unexpected payload metadata: %+v", payload)
	}
	if len(payload.Groups) != 2 || len(payload.Series) != 2 {
		t.Fatalf("expected two distinct groups and series: %+v", payload)
	}
	group := payload.Groups[0]
	if group.GroupID != "a" || group.GroupName != "Shared name" || group.Requests != 12 {
		t.Fatalf("unexpected summary: %+v", group)
	}
	assertPerformanceNumber(t, "summary TPS", group.TPS, 12.0/86400)
	assertPerformanceNumber(t, "request-weighted latency", group.AvgLatencyMs, 4800.0/12)
	assertPerformanceNumber(t, "request-weighted success", group.SuccessRate, 6.0/12)
	assertPerformanceOptional(t, "observation-weighted first token", group.AvgFirstTokenMs, 80)
	for i, series := range payload.Series {
		if series.GroupID != payload.Groups[i].GroupID || len(series.Points) != 24 {
			t.Fatalf("unexpected series: %+v", series)
		}
		for j, point := range series.Points {
			if !point.Timestamp.Equal(payload.WindowStart.Add(time.Duration(j)*time.Hour)) || !point.Timestamp.Before(now) {
				t.Fatalf("group %s bucket %d timestamp = %s", series.GroupID, j, point.Timestamp)
			}
			if point.Requests == 0 && (point.FailedRequests != 0 || point.TPS != 0 || point.AvgLatencyMs != nil || point.AvgFirstTokenMs != nil || point.SuccessRate != nil) {
				t.Fatalf("empty bucket must contain zero traffic and missing observations: %+v", point)
			}
		}
	}
	point := payload.Series[0].Points[2]
	if point.Requests != 9 || point.FailedRequests != 6 {
		t.Fatalf("bucket requests = %d, failures = %d, want 9 and 6", point.Requests, point.FailedRequests)
	}
	if payload.Series[0].Points[0].FailedRequests != 0 || payload.Series[1].Points[2].FailedRequests != 1 {
		t.Fatal("fully successful and fully failed buckets have incorrect failure counts")
	}
	assertPerformanceNumber(t, "bucket TPS", point.TPS, 9.0/3600)
	assertPerformanceOptional(t, "bucket latency", point.AvgLatencyMs, 500)
	assertPerformanceOptional(t, "bucket first token", point.AvgFirstTokenMs, 100)
	assertPerformanceOptional(t, "bucket success", point.SuccessRate, 1.0/3)
	if payload.Series[0].Points[23].AvgFirstTokenMs != nil || payload.Groups[1].AvgFirstTokenMs != nil {
		t.Fatal("missing first-token observations must remain nil")
	}
	assertPerformanceOptional(t, "observed zero latency", payload.Series[1].Points[2].AvgLatencyMs, 0)
	assertPerformanceOptional(t, "observed zero success", payload.Series[1].Points[2].SuccessRate, 0)
}

func TestModelPerformanceJSONEmptyAndMissingObservations(t *testing.T) {
	now := time.Date(2026, time.October, 19, 16, 37, 0, 0, time.UTC)
	empty := buildModelPerformancePayload("test-model", now, nil)
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["groups"]) != "[]" || string(fields["series"]) != "[]" {
		t.Fatalf("empty response must contain arrays: %s", encoded)
	}
	payload := buildModelPerformancePayload("test-model", now, []modelPerformanceBucket{
		{GroupID: "__public", GroupName: "公共", Index: 1, Requests: 1, FirstTokenCount: 1},
	})
	for _, test := range []struct {
		index int
		want  string
	}{
		{index: 0, want: "null"},
		{index: 1, want: "0"},
	} {
		encoded, err := json.Marshal(payload.Series[0].Points[test.index])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"avg_latency_ms", "avg_first_token_ms", "success_rate"} {
			if string(fields[name]) != test.want {
				t.Errorf("bucket %d %s = %s, want %s", test.index, name, fields[name], test.want)
			}
		}
	}
}

func assertPerformanceNumber(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("%s = %g, want %g", label, got, want)
	}
}

func assertPerformanceOptional(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s is nil, want %g", label, want)
		return
	}
	assertPerformanceNumber(t, label, *got, want)
}
