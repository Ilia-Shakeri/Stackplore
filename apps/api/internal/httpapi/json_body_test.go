package httpapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type strictBodyFixture struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func TestDecodeJSONBodyAcceptsOneStrictObject(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://stackplore.test/api/v1/test", strings.NewReader(`{"name":"one","role":"viewer"}`))
	request.Header.Set("Content-Type", "application/json")
	var got strictBodyFixture
	if err := decodeJSONBody(request, 128, &got); err != nil {
		t.Fatalf("decodeJSONBody() error = %v", err)
	}
	if got.Name != "one" || got.Role != "viewer" {
		t.Fatalf("decoded body = %#v", got)
	}
}

func TestDecodeJSONBodyRejectsUnsafeShapes(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		maximum     int64
		want        error
	}{
		{"empty", "", "application/json", 128, errJSONBodyInvalid},
		{"unknown", `{"name":"one","extra":true}`, "application/json", 128, errJSONBodyInvalid},
		{"duplicate", `{"name":"one","name":"two"}`, "application/json", 128, errJSONBodyInvalid},
		{"nested duplicate", `{"name":"one","role":{"x":1,"x":2}}`, "application/json", 128, errJSONBodyInvalid},
		{"trailing", `{"name":"one"}{}`, "application/json", 128, errJSONBodyInvalid},
		{"array", `[]`, "application/json", 128, errJSONBodyInvalid},
		{"bad media", `{}`, "text/plain", 128, errJSONMediaType},
		{"media parameter", `{}`, "application/json; charset=utf-8", 128, errJSONMediaType},
		{"large declared", `{}`, "application/json", 1, errJSONBodyInvalid},
		{"large streamed", `{"name":"long"}`, "application/json", 4, errJSONBodyTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://stackplore.test/api/v1/test", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if test.name == "large streamed" {
				request.ContentLength = -1
			}
			var target strictBodyFixture
			if err := decodeJSONBody(request, test.maximum, &target); !errors.Is(err, test.want) {
				t.Fatalf("error = %v; want %v", err, test.want)
			}
		})
	}
}

func TestDecodeJSONBodySealsReadFailures(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://stackplore.test/api/v1/test", nil)
	request.Header.Set("Content-Type", "application/json")
	request.Body = io.NopCloser(failingBodyReader{})
	request.ContentLength = -1
	var target strictBodyFixture
	if err := decodeJSONBody(request, 128, &target); !errors.Is(err, errJSONBodyInvalid) {
		t.Fatalf("error = %v", err)
	}
}

type failingBodyReader struct{}

func (failingBodyReader) Read([]byte) (int, error) { return 0, errors.New("raw read failure") }
