package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRewardBanSkipsProtectedAndDisabledUsers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		role        string
		enabled     bool
		permissions map[string]bool
	}{
		{"admin", "admin", true, nil},
		{"operator", "operator", true, nil},
		{"permissioned user", "user", true, map[string]bool{"users.read": true}},
		{"already disabled", "user", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &userAuthorizationTestTx{users: map[string]userAccess{"7": {accountContext: accountContext{userID: "7", role: tc.role, permissions: tc.permissions}, enabled: tc.enabled}}}
			banned, err := (&Service{}).autoBanRewardUserTx(context.Background(), tx, "7", "unused", 1, nil)
			if err != nil || banned {
				t.Fatalf("banned=%v err=%v", banned, err)
			}
			if len(tx.calls) != 1 {
				t.Fatalf("protected user triggered mutation: %v", tx.calls)
			}
		})
	}
}

func TestRewardUnbanRejectsMalformedRequestBeforeDatabase(t *testing.T) {
	for _, tc := range []struct{ userID, body string }{
		{"7", `{}`},
		{"7", `{"case_id":"not-a-uuid","reason":"reviewed"}`},
		{"7", `{"case_id":"00000000-0000-0000-0000-000000000001","reason":" "}`},
		{"7", `{"case_id":"00000000-0000-0000-0000-000000000001","reason":"` + strings.Repeat("x", 501) + `"}`},
		{"0", `{"case_id":"00000000-0000-0000-0000-000000000001","reason":"reviewed"}`},
		{"invalid", `{"case_id":"00000000-0000-0000-0000-000000000001","reason":"reviewed"}`},
	} {
		r := httptest.NewRequest(http.MethodPost, "/admin/reward-risk/users/"+tc.userID+"/unban", strings.NewReader(tc.body))
		r.SetPathValue("id", tc.userID)
		w := httptest.NewRecorder()
		(&Service{}).unbanRewardUser(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("user=%s body=%s status=%d", tc.userID, tc.body, w.Code)
		}
	}
}
