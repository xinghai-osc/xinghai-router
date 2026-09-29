package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseRequestFormAcceptsLargeBodies(t *testing.T) {
	const merchantKey = "fake-test-merchant-key"
	form := url.Values{"pid": {"test-merchant"}, "extra": {strings.Repeat("x", (10<<20)+1)}}
	form.Set("sign", epaySign(form, merchantKey))
	body := form.Encode()
	for _, unknownLength := range []bool{false, true} {
		t.Run(fmt.Sprintf("unknown=%v", unknownLength), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/epay/notify", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
			if unknownLength {
				r.ContentLength = -1
			}
			if err := parseRequestForm(httptest.NewRecorder(), r, time.Second); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.PostForm, form) || !reflect.DeepEqual(r.Form, form) {
				t.Fatal("large form values changed")
			}
			if !equalSecret(r.Form.Get("sign"), epaySign(r.Form, merchantKey)) {
				t.Fatal("large form signature changed")
			}
		})
	}
}

func TestParseRequestFormPreservesParsing(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch} {
		for _, contentType := range []string{"application/x-www-form-urlencoded", "application/x-www-form-urlencoded; charset=utf-8", "application/json", "", "invalid;"} {
			t.Run(method+"/"+contentType, func(t *testing.T) {
				newRequest := func() *http.Request {
					r := httptest.NewRequest(method, "/epay/notify?pid=query&query=kept", strings.NewReader("pid=body&pid=second&value=a%2Bb+c"))
					r.Header.Set("Content-Type", contentType)
					return r
				}
				want, got := newRequest(), newRequest()
				wantErr := want.ParseForm()
				gotErr := parseRequestForm(httptest.NewRecorder(), got, time.Second)
				if (wantErr == nil) != (gotErr == nil) || !reflect.DeepEqual(got.Form, want.Form) || !reflect.DeepEqual(got.PostForm, want.PostForm) {
					t.Fatalf("form=%v want=%v post=%v want=%v error=%v want=%v", got.Form, want.Form, got.PostForm, want.PostForm, gotErr, wantErr)
				}
			})
		}
	}
	for _, body := range []string{"pid=%zz", "pid=value;extra=value"} {
		r := httptest.NewRequest(http.MethodPost, "/epay/notify", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := parseRequestForm(httptest.NewRecorder(), r, time.Second); err == nil {
			t.Fatalf("accepted malformed form %q", body)
		}
	}
}

func TestReauthenticationValidatesLargeBody(t *testing.T) {
	s := &Service{limiter: newLimiter(60)}
	r := httptest.NewRequest(http.MethodPost, "/auth/reauthenticate", strings.NewReader(strings.Repeat(" ", 4097)+`{"password":""}`))
	r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{userID: "test-user"}))
	w := httptest.NewRecorder()
	s.reauthenticate(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "a password between 8 and 72 characters is required") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
}
