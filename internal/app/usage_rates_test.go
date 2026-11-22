package app

import (
	"math"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestUsageRateWindow(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	first := now.Add(-5 * time.Minute)
	future := now.Add(time.Minute)
	for _, tc := range []struct {
		name       string
		start, end string
		first      *time.Time
		requests   int64
		tokens     int64
		rpm, tpm   float64
		filtered   bool
	}{
		{name: "selected window", start: "2026-01-02T11:50:00Z", end: "2026-01-02T12:00:00Z", first: &first, requests: 10, tokens: 1000, rpm: 1, tpm: 100, filtered: true},
		{name: "future end clamped", start: "2026-01-02T11:50:00Z", end: "2026-01-03T12:00:00Z", first: &first, requests: 10, tokens: 1000, rpm: 1, tpm: 100, filtered: true},
		{name: "missing end", start: "2026-01-02T11:50:00Z", first: &first, requests: 10, tokens: 1000, rpm: 1, tpm: 100, filtered: true},
		{name: "missing start uses first match", end: "2026-01-02T11:57:00Z", first: &first, requests: 10, tokens: 1000, rpm: 5, tpm: 500, filtered: true},
		{name: "missing range", first: &first, requests: 10, tokens: 1000, rpm: 2, tpm: 200},
		{name: "minimum minute", start: "2026-01-02T11:59:30Z", requests: 10, tokens: 1000, rpm: 10, tpm: 1000, filtered: true},
		{name: "fractional minutes", start: "2026-01-02T11:58:30Z", requests: 3, tokens: 150, rpm: 2, tpm: 100, filtered: true},
		{name: "equal bounds", start: "2026-01-02T12:00:00Z", end: "2026-01-02T12:00:00Z", requests: 10, tokens: 1000, rpm: 10, tpm: 1000, filtered: true},
		{name: "future range", start: "2026-01-03T12:00:00Z", end: "2026-01-04T12:00:00Z", requests: 10, tokens: 1000, filtered: true},
		{name: "reversed range", start: "2026-01-02T11:59:00Z", end: "2026-01-02T11:58:00Z", requests: 10, tokens: 1000, filtered: true},
		{name: "empty range", start: "2026-01-02T11:50:00Z", filtered: true},
		{name: "no matching record"},
		{name: "future first record", first: &future, requests: 10, tokens: 1000},
		{name: "invalid dates retain defaults", start: "invalid", end: "invalid", first: &first, requests: 10, tokens: 1000, rpm: 2, tpm: 200},
		{name: "offset timestamps", start: " 2026-01-02T19:50:00+08:00 ", end: "2026-01-02T20:00:00+08:00", requests: 10, tokens: 1000, rpm: 1, tpm: 100, filtered: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := url.Values{"start": {tc.start}, "end": {tc.end}}
			r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
			window := newUsageRateWindow(r, now)
			if window.end.After(now) || window.filtered != tc.filtered {
				t.Fatalf("unexpected window: %+v", window)
			}
			rpm, tpm := window.rates(tc.requests, tc.tokens, tc.first)
			if math.IsNaN(rpm) || math.IsInf(rpm, 0) || rpm < 0 || math.Abs(rpm-tc.rpm) > 1e-9 {
				t.Errorf("rpm=%v, want %v", rpm, tc.rpm)
			}
			if math.IsNaN(tpm) || math.IsInf(tpm, 0) || tpm < 0 || math.Abs(tpm-tc.tpm) > 1e-9 {
				t.Errorf("tpm=%v, want %v", tpm, tc.tpm)
			}
		})
	}
}
