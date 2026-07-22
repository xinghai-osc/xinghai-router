package app

import "testing"

func TestPasswordChangeAllowedPath(t *testing.T) {
	allowed := []string{"/account/me", "/account/password", "/auth/logout"}
	for _, path := range allowed {
		if !passwordChangeAllowedPath(path) {
			t.Fatalf("%s must be allowed while must_change_password is set", path)
		}
	}
	blocked := []string{"/account/keys", "/admin/users", "/account/usage", "/account/preferences"}
	for _, path := range blocked {
		if passwordChangeAllowedPath(path) {
			t.Fatalf("%s must be blocked while must_change_password is set", path)
		}
	}
}
