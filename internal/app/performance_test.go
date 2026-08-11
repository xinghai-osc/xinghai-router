package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelPerformanceRequiresModel(t *testing.T) {
	s := &Service{}
	recorder := httptest.NewRecorder()
	s.modelPerformance(recorder, httptest.NewRequest(http.MethodGet, "/model-performance", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}
