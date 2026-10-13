package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/sourcesettings"
)

type fakeSourceSettings struct {
	SourceSettings
	listCalls     int
	previewCalls  int
	updateCalls   int
	configuration domain.SourceConfiguration
}

func (settings *fakeSourceSettings) List(_ context.Context, page domain.PageRequest) (domain.SourceConfigurationPage, error) {
	settings.listCalls++
	return domain.SourceConfigurationPage{Configurations: []domain.SourceConfiguration{settings.configuration}, Page: domain.PageInfo{Limit: page.Limit}}, nil
}
func (*fakeSourceSettings) ProfileAvailable(domain.SourceConfiguration) bool { return true }
func (settings *fakeSourceSettings) Preview(_ context.Context, actor auth.Principal, id domain.SourceID, version int64, enabled bool) (sourcesettings.Preview, error) {
	settings.previewCalls++
	if actor.Role != auth.RoleAdministrator || id != settings.configuration.Source.ID || version != settings.configuration.Version || enabled {
		return sourcesettings.Preview{}, sourcesettings.ErrInvalid
	}
	return sourcesettings.Preview{Configuration: settings.configuration, RequestedEnabled: enabled, ConfirmationToken: strings.Repeat("A", 43), ExpiresAt: settings.configuration.CreatedAt.Add(2 * time.Minute)}, nil
}
func (settings *fakeSourceSettings) Update(_ context.Context, actor auth.Principal, id domain.SourceID, version int64, enabled bool, token string, grant auth.RecentGrantUse, request domain.RequestID) (domain.SourceConfiguration, error) {
	settings.updateCalls++
	if actor.Role != auth.RoleAdministrator || grant.Purpose != auth.RecentAuthPurposeSourceUpdate || id != "source_demo" || version != 1 || enabled || token != strings.Repeat("A", 43) || request.Validate() != nil {
		return domain.SourceConfiguration{}, sourcesettings.ErrInvalid
	}
	configuration := settings.configuration
	configuration.Enabled = false
	configuration.Version++
	return configuration, nil
}

type sourceAuthentication struct {
	*testAuthentication
	prepare func(context.Context, auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error)
}

func (authentication sourceAuthentication) PrepareSourceMutation(ctx context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error) {
	return authentication.prepare(ctx, input)
}

func sourceSettingsHTTPFixture(t *testing.T, bodyRead ...*bool) (http.Handler, *fakeSourceSettings, *auth.Role) {
	t.Helper()
	at := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	role := auth.RoleAdministrator
	identity := func() auth.Authentication {
		return auth.Authentication{Principal: auth.Principal{UserID: "usr_admin", SessionID: "sess_admin", Role: role, AuthRevision: 1}}
	}
	settings := &fakeSourceSettings{configuration: domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_demo", Provider: domain.ProviderDemo, Name: "Demo", Synthetic: true},
		ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}}
	authentication := sourceAuthentication{testAuthentication: &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity(), nil },
		authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity(), nil }},
		prepare: func(_ context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error) {
			if len(bodyRead) > 0 && !*bodyRead[0] {
				t.Fatal("recent proof prepared before bounded body read")
			}
			if role != auth.RoleAdministrator {
				return auth.Authentication{}, auth.RecentGrantUse{}, auth.ErrAuthorizationDenied
			}
			if input.Purpose != auth.RecentAuthPurposeSourceUpdate || input.RecentProof != strings.Repeat("A", 43) {
				return auth.Authentication{}, auth.RecentGrantUse{}, auth.ErrRecentAuthRejected
			}
			return identity(), auth.RecentGrantUse{Actor: identity().Principal, Purpose: input.Purpose, ConsumedAt: at}, nil
		}}
	handler, _, _ := makeHandler(t, func(options *Options) { options.Authentication = authentication; options.SourceSettings = settings })
	return handler, settings, &role
}

type observedBodyReader struct {
	reader io.Reader
	read   *bool
}

func (body observedBodyReader) Read(bytes []byte) (int, error) {
	*body.read = true
	return body.reader.Read(bytes)
}

func TestSourceMutationPreparesRecentProofAfterBodyRead(t *testing.T) {
	read := false
	handler, settings, _ := sourceSettingsHTTPFixture(t, &read)
	body := `{"enabled":false,"confirmation_token":"` + strings.Repeat("A", 43) + `"}`
	request := makeRequest(http.MethodPatch, "/api/v1/settings/sources/source_demo", "198.51.100.10:4000")
	request.Body = io.NopCloser(observedBodyReader{reader: strings.NewReader(body), read: &read})
	request.ContentLength = int64(len(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, strings.Repeat("A", 43))
	request.Header.Set(recentAuthHeaderName, strings.Repeat("A", 43))
	request.Header.Set("If-Match", `"1"`)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	if response := serve(handler, request); response.Code != 200 || !read || settings.updateCalls != 1 {
		t.Fatal("bounded source body workflow failed")
	}
}

func TestSourceSettingsRejectsViewerAndInvalidPageBeforeRegistryRead(t *testing.T) {
	handler, settings, role := sourceSettingsHTTPFixture(t)
	*role = auth.RoleViewer
	if response := authenticatedProductRequest(handler, "/api/v1/settings/sources"); response.Code != 403 || settings.listCalls != 0 {
		t.Fatal("viewer read source registry")
	}
	*role = auth.RoleAdministrator
	if response := authenticatedProductRequest(handler, "/api/v1/settings/sources?limit=101"); response.Code != 400 || settings.listCalls != 0 {
		t.Fatal("unbounded source page accepted")
	}
	response := authenticatedProductRequest(handler, "/api/v1/settings/sources")
	var page sourceSettingsPage
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Items) != 1 || page.Items[0].ID != "source_demo" {
		t.Fatal("source registry page failed")
	}
}

func TestSourcePreviewRejectsEndpointsMissingVersionAndUnknownFields(t *testing.T) {
	handler, settings, _ := sourceSettingsHTTPFixture(t)
	for _, body := range []string{`{"enabled":false,"endpoint":"https://example.test"}`, `{"name":"other"}`, `{"enabled":null}`} {
		request := makeRequest(http.MethodPost, "/api/v1/settings/sources/source_demo/preview", "198.51.100.10:4000")
		request.Body = http.NoBody
		request = withJSONBody(request, body)
		request.Header.Set("Origin", "http://stackplore.test")
		request.Header.Set(csrfHeaderName, strings.Repeat("A", 43))
		request.Header.Set("If-Match", `"1"`)
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
		if response := serve(handler, request); response.Code != 400 || settings.previewCalls != 0 {
			t.Fatal("unsafe source preview body accepted")
		}
	}
}

func withJSONBody(request *http.Request, body string) *http.Request {
	clone := request.Clone(request.Context())
	clone.Body = io.NopCloser(strings.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.Header.Set("Content-Type", "application/json")
	return clone
}

func TestSourceMutationRequiresRecentProofAndUsesExactVersion(t *testing.T) {
	handler, settings, _ := sourceSettingsHTTPFixture(t)
	request := makeRequest(http.MethodPatch, "/api/v1/settings/sources/source_demo", "198.51.100.10:4000")
	request = withJSONBody(request, `{"enabled":false,"confirmation_token":"`+strings.Repeat("A", 43)+`"}`)
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, strings.Repeat("A", 43))
	request.Header.Set("If-Match", `"1"`)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	if response := serve(handler, request); response.Code != 403 || settings.updateCalls != 0 {
		t.Fatal("source mutation bypassed recent proof")
	}
	request = withJSONBody(request, `{"enabled":false,"confirmation_token":"`+strings.Repeat("A", 43)+`"}`)
	request.Header.Set(recentAuthHeaderName, strings.Repeat("A", 43))
	response := serve(handler, request)
	if response.Code != 200 || response.Header().Get("ETag") != `"2"` || settings.updateCalls != 1 {
		t.Fatal("exact source mutation failed")
	}
}
