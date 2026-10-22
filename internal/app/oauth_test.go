package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type githubAPITransport struct{ base string }

func (t githubAPITransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	u.Host = t.base
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return http.DefaultTransport.RoundTrip(req2)
}

func testGithubService(t *testing.T, server *httptest.Server) *Service {
	t.Helper()
	return &Service{httpClient: &http.Client{Transport: githubAPITransport{base: server.Listener.Addr().String()}}}
}

func TestGithubFetchUserUsesOnlyPrimaryVerifiedEmail(t *testing.T) {
	cases := []struct {
		name         string
		userEmail    string
		emails       []map[string]any
		wantEmail    string
		wantVerified bool
	}{
		{
			name:      "primary verified email wins over profile email",
			userEmail: "public@example.com",
			emails: []map[string]any{
				{"email": "primary@example.com", "primary": true, "verified": true},
				{"email": "public@example.com", "primary": false, "verified": true},
			},
			wantEmail:    "primary@example.com",
			wantVerified: true,
		},
		{
			name:      "unverified primary is rejected even if it matches a profile email",
			userEmail: "victim@example.com",
			emails: []map[string]any{
				{"email": "victim@example.com", "primary": true, "verified": false},
			},
			wantEmail:    "",
			wantVerified: false,
		},
		{
			name:      "no primary verified email means no verified email",
			userEmail: "victim@example.com",
			emails: []map[string]any{
				{"email": "secondary@example.com", "primary": false, "verified": true},
				{"email": "victim@example.com", "primary": false, "verified": false},
			},
			wantEmail:    "",
			wantVerified: false,
		},
		{
			name:         "profile email is never trusted without email verification",
			userEmail:    "victim@example.com",
			emails:       []map[string]any{},
			wantEmail:    "",
			wantVerified: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/user":
					writeJSON(w, 200, map[string]any{"id": 12345, "login": "legion", "name": "Legion", "email": tc.userEmail, "avatar_url": "https://example.com/a.png"})
				case "/user/emails":
					writeJSON(w, 200, tc.emails)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			s := testGithubService(t, server)
			id, email, name, avatar, verified, err := s.githubFetchUser(context.Background(), "token")
			if err != nil {
				t.Fatalf("githubFetchUser() error = %v", err)
			}
			if id != "12345" || name != "Legion" || avatar != "https://example.com/a.png" {
				t.Fatalf("user fields id=%q name=%q avatar=%q", id, name, avatar)
			}
			if email != tc.wantEmail || verified != tc.wantVerified {
				t.Fatalf("email=%q verified=%v, want email=%q verified=%v", email, verified, tc.wantEmail, tc.wantVerified)
			}
		})
	}
}

func TestGithubFetchUserRejectsEmailsEndpointFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"forbidden", http.StatusForbidden, `{"message":"forbidden"}`},
		{"server-error", http.StatusInternalServerError, `{"message":"failure"}`},
		{"invalid-json", http.StatusOK, `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/user" {
					writeJSON(w, http.StatusOK, map[string]any{"id": 1, "login": "fake-user", "email": "victim@example.test"})
					return
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			id, email, _, _, verified, err := testGithubService(t, server).githubFetchUser(t.Context(), "fake-token")
			if err == nil || email != "" || verified {
				t.Fatalf("identity=%q email=%q verified=%v err=%v", id, email, verified, err)
			}
		})
	}
}

func TestGithubFetchUserRejectsInvalidIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			writeJSON(w, http.StatusOK, map[string]any{"message": "API rate limit exceeded"})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{})
	}))
	defer server.Close()
	s := testGithubService(t, server)
	if _, _, _, _, _, err := s.githubFetchUser(context.Background(), "token"); err == nil {
		t.Fatal("githubFetchUser() accepted an identity with id 0")
	}
}

func TestGithubFetchUserRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"id": "not-a-number"})
	}))
	defer server.Close()
	s := testGithubService(t, server)
	if _, _, _, _, _, err := s.githubFetchUser(context.Background(), "token"); err == nil {
		t.Fatal("githubFetchUser() expected error for invalid user JSON")
	}
}

func TestGithubExchangeTokenRejectsInvalidResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"valid", 200, `{"access_token":"fake-token"}`, "fake-token"},
		{"empty", 200, `{}`, ""},
		{"whitespace", 200, `{"access_token":"   "}`, ""},
		{"error", 200, `{"error_description":"secret-upstream-detail"}`, ""},
		{"forbidden", 403, `{"access_token":"fake-token"}`, ""},
		{"invalid-json", 200, `{`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/login/oauth/access_token" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			s := testGithubService(t, server)
			token, err := s.githubExchangeToken(t.Context(), &oauthProviderConfig{ClientID: "fake", ClientSecret: "fake-secret"}, "fake-code", "https://router.example/callback")
			if token != tc.want || ((err == nil) != (tc.want != "")) {
				t.Fatalf("token=%q err=%v", token, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-upstream-detail") {
				t.Fatal("upstream secret reflected in error")
			}
		})
	}
}

func TestGithubFetchUserRejectsHTTPErrorEvenWithIdentity(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, status, map[string]any{"id": 12345, "login": "fake-user"})
			}))
			defer server.Close()
			if _, _, _, _, _, err := testGithubService(t, server).githubFetchUser(t.Context(), "fake-token"); err == nil {
				t.Fatal("accepted GitHub error response")
			}
		})
	}
}

func TestGithubFetchUserRejectsEmptyAccessToken(t *testing.T) {
	if _, _, _, _, _, err := (&Service{}).githubFetchUser(t.Context(), " "); err == nil {
		t.Fatal("empty access token accepted")
	}
}

func TestGithubFetchUserDoesNotRequireProfileLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			writeJSON(w, 200, map[string]any{"id": 42, "name": "Fake"})
			return
		}
		writeJSON(w, 200, []map[string]any{{"email": "valid@example.test", "primary": true, "verified": true}})
	}))
	defer server.Close()
	id, email, _, _, verified, err := testGithubService(t, server).githubFetchUser(t.Context(), "fake-token")
	if err != nil || id != "42" || email != "valid@example.test" || !verified {
		t.Fatalf("identity=%q email=%q verified=%v err=%v", id, email, verified, err)
	}
}
