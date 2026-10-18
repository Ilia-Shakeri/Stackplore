package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSameOriginNeedsExactOriginAndFetchMetadata(t *testing.T) {
	origin := Origin{Scheme: "https", Authority: "stackplore.test"}
	tests := []struct {
		name   string
		values []string
		fetch  []string
		want   bool
	}{
		{"exact", []string{"https://stackplore.test"}, []string{"same-origin"}, true},
		{"fetch omitted", []string{"https://stackplore.test"}, nil, true},
		{"missing", nil, nil, false},
		{"duplicate", []string{"https://stackplore.test", "https://stackplore.test"}, nil, false},
		{"other scheme", []string{"http://stackplore.test"}, nil, false},
		{"other host", []string{"https://other.test"}, nil, false},
		{"null", []string{"null"}, nil, false},
		{"cross site", []string{"https://stackplore.test"}, []string{"cross-site"}, false},
		{"duplicate fetch", []string{"https://stackplore.test"}, []string{"same-origin", "same-origin"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://stackplore.test/api/v1/auth/login", nil)
			request.Header.Del("Origin")
			for _, value := range test.values {
				request.Header.Add("Origin", value)
			}
			for _, value := range test.fetch {
				request.Header.Add("Sec-Fetch-Site", value)
			}
			if got := validSameOrigin(request, origin); got != test.want {
				t.Fatalf("validSameOrigin() = %t; want %t", got, test.want)
			}
		})
	}
}

func TestAuthCookieFlagsAndClearing(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	response := httptest.NewRecorder()
	setAuthCookie(response, sessionCookieName, strings.Repeat("A", 43), expiresAt, time.Now().UTC(), true, true)
	result := response.Result()
	cookies := result.Cookies()
	_ = result.Body.Close()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.Path != authCookiePath || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Domain != "" || cookie.MaxAge < 1 {
		t.Fatal("cookie flags do not match the security policy")
	}

	cleared := httptest.NewRecorder()
	clearAuthCookie(cleared, sessionCookieName, false, true)
	result = cleared.Result()
	cookies = result.Cookies()
	_ = result.Body.Close()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 || cookies[0].Value != "" || cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != authCookiePath {
		t.Fatal("cleared cookie does not match the expiry policy")
	}
}

func TestProofReadersRejectDuplicatesAndLooseValues(t *testing.T) {
	valid := strings.Repeat("A", 43)
	request := httptest.NewRequest(http.MethodPost, "http://stackplore.test/api/v1/auth/logout", nil)
	request.Header.Set(csrfHeaderName, valid)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: valid})
	if got, ok := readSingleHeader(request, csrfHeaderName); !ok || got != valid {
		t.Fatal("valid header was rejected")
	}
	if got, ok := readCookieValue(request, sessionCookieName); !ok || got != valid {
		t.Fatal("valid cookie was rejected")
	}
	request.Header.Add(csrfHeaderName, valid)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: valid})
	if _, ok := readSingleHeader(request, csrfHeaderName); ok {
		t.Fatal("duplicate header accepted")
	}
	if _, ok := readCookieValue(request, sessionCookieName); ok {
		t.Fatal("duplicate cookie accepted")
	}
}
