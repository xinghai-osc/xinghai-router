package app

import (
	"net/http"
	"strings"
	"time"
)

type usageRateWindow struct {
	start    *time.Time
	end      time.Time
	filtered bool
}

func newUsageRateWindow(r *http.Request, now time.Time) usageRateWindow {
	window := usageRateWindow{end: now}
	if start, err := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("start"))); err == nil {
		window.start = &start
		window.filtered = true
	}
	if end, err := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("end"))); err == nil {
		window.filtered = true
		if end.Before(window.end) {
			window.end = end
		}
	}
	return window
}

func (window usageRateWindow) rates(requests, tokens int64, first *time.Time) (float64, float64) {
	start := window.start
	if start == nil {
		start = first
	}
	if requests <= 0 || start == nil || window.end.Before(*start) {
		return 0, 0
	}
	minutes := max(window.end.Sub(*start).Minutes(), 1)
	return float64(requests) / minutes, float64(max(tokens, 0)) / minutes
}
