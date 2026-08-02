package httpapi

import (
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName    = "stackplore_session"
	preauthCookieName    = "stackplore_preauth"
	preauthHeaderName    = "X-Stackplore-Preauth"
	csrfHeaderName       = "X-Stackplore-CSRF"
	recentAuthHeaderName = "X-Stackplore-Recent-Auth"
	authCookiePath       = "/api/v1"
)

func validSameOrigin(r *http.Request, origin Origin) bool {
	if r == nil || origin.Scheme != "http" && origin.Scheme != "https" || origin.Authority == "" {
		return false
	}
	values := r.Header.Values("Origin")
	if len(values) != 1 || values[0] != origin.Scheme+"://"+origin.Authority {
		return false
	}
	fetchSites := r.Header.Values("Sec-Fetch-Site")
	if len(fetchSites) > 1 || len(fetchSites) == 1 && fetchSites[0] != "same-origin" {
		return false
	}
	return true
}

func readSingleHeader(r *http.Request, name string) (string, bool) {
	if r == nil {
		return "", false
	}
	values := r.Header.Values(name)
	return singleProofValue(values)
}

func readCookieValue(r *http.Request, name string) (string, bool) {
	if r == nil || name == "" || strings.ContainsAny(name, "\x00\r\n;= ") {
		return "", false
	}
	cookies := r.CookiesNamed(name)
	if len(cookies) != 1 {
		return "", false
	}
	return singleProofValue([]string{cookies[0].Value})
}

func singleProofValue(values []string) (string, bool) {
	if len(values) != 1 || len(values[0]) != 43 {
		return "", false
	}
	for index := range len(values[0]) {
		character := values[0][index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return "", false
	}
	return values[0], true
}

func setAuthCookie(w http.ResponseWriter, name, value string, expiresAt, now time.Time, secure bool, httpOnly bool) {
	remaining := expiresAt.Sub(now)
	maxAge := int(remaining.Seconds())
	if remaining > 0 && maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     authCookiePath,
		Expires:  expiresAt.UTC(),
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookie(w http.ResponseWriter, name string, secure bool, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     authCookiePath,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteLaxMode,
	})
}
