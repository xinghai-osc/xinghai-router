package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionAffinityHandlersWithoutACache(t *testing.T) {
	service := &Service{}

	list := httptest.NewRecorder()
	service.listSessionAffinity(list, httptest.NewRequest(http.MethodGet, "/admin/session-affinity", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"data":[]`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	del := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/admin/session-affinity/ses-1", nil)
	request.SetPathValue("session", "ses-1")
	service.deleteSessionAffinity(del, request)
	if del.Code != http.StatusServiceUnavailable {
		t.Fatalf("delete status=%d body=%s", del.Code, del.Body.String())
	}

	clear := httptest.NewRecorder()
	service.clearSessionAffinity(clear, httptest.NewRequest(http.MethodDelete, "/admin/session-affinity", nil))
	if clear.Code != http.StatusServiceUnavailable {
		t.Fatalf("clear status=%d body=%s", clear.Code, clear.Body.String())
	}
}

func TestSessionAffinityDeleteValidatesAndReportsMissingAssignments(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(0, 0)}

	for name, session := range map[string]string{
		"empty":     " ",
		"oversized": strings.Repeat("x", maxSessionIDLength+1),
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/admin/session-affinity/session", nil)
		request.SetPathValue("session", session)
		service.deleteSessionAffinity(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s session: status=%d body=%s", name, recorder.Code, recorder.Body.String())
		}
	}

	// A session the gateway never pinned is a not-found, and the scope query
	// does not turn that into a deletion.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/admin/session-affinity/ses-unknown?user_id=user-1", nil)
	request.SetPathValue("session", "ses-unknown")
	service.deleteSessionAffinity(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown session: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSessionAffinityClearWithoutAssignmentsWritesNothing(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(0, 0)}
	recorder := httptest.NewRecorder()
	service.clearSessionAffinity(recorder, httptest.NewRequest(http.MethodDelete, "/admin/session-affinity", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"removed":0`) {
		t.Fatalf("clear status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestShortSessionPrefixBoundsTheRenderedSession(t *testing.T) {
	if got := shortSessionPrefix("ses-short"); got != "ses-short" {
		t.Fatalf("short session = %q", got)
	}
	long := strings.Repeat("a", 64)
	got := shortSessionPrefix(long)
	if len(got) != 16 || !strings.HasPrefix(long, got) {
		t.Fatalf("long session = %q", got)
	}
}
