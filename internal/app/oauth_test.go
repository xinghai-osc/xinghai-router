package app

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestGithubFetchUserIgnoresEmailsEndpointFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			writeJSON(w, 200, map[string]any{"id": 1, "login": "legion", "email": "victim@example.com"})
		case "/user/emails":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	s := testGithubService(t, server)
	_, email, _, _, verified, err := s.githubFetchUser(context.Background(), "token")
	if err != nil {
		t.Fatalf("githubFetchUser() error = %v", err)
	}
	if email != "" || verified {
		t.Fatalf("email=%q verified=%v, want empty unverified", email, verified)
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