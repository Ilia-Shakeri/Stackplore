package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/kubernetes"
)

type staticClusterCollector struct{}

type auditReaderFunc func(context.Context, domain.PageRequest) (domain.AuditPage, error)

type alertReaderFunc func(context.Context, domain.SourceID, domain.PageRequest) (domain.AlertPage, error)

type notificationReaderFunc func(context.Context, domain.PageRequest) (domain.NotificationPage, error)
type alertManagerFunc func(context.Context, domain.AlertID, domain.UserID, domain.RequestID) (domain.Alert, error)
type notificationTesterFunc func(context.Context, domain.UserID, domain.RequestID) (domain.Notification, error)

func (read auditReaderFunc) ListAuditEntries(ctx context.Context, page domain.PageRequest) (domain.AuditPage, error) {
	return read(ctx, page)
}

func (read alertReaderFunc) ListAlerts(ctx context.Context, sourceID domain.SourceID, page domain.PageRequest) (domain.AlertPage, error) {
	return read(ctx, sourceID, page)
}

func (read notificationReaderFunc) ListNotifications(ctx context.Context, page domain.PageRequest) (domain.NotificationPage, error) {
	return read(ctx, page)
}

func (test notificationTesterFunc) RequestTest(ctx context.Context, userID domain.UserID, requestID domain.RequestID) (domain.Notification, error) {
	return test(ctx, userID, requestID)
}

func (manage alertManagerFunc) Acknowledge(ctx context.Context, alertID domain.AlertID, userID domain.UserID, requestID domain.RequestID) (domain.Alert, error) {
	return manage(ctx, alertID, userID, requestID)
}

func (alertManagerFunc) ListRules(context.Context, domain.PageRequest) (domain.AlertRulePage, error) {
	return domain.AlertRulePage{}, alerting.ErrManagementUnavailable
}

func (alertManagerFunc) CreateRule(context.Context, alerting.RuleInput, domain.UserID, domain.RequestID) (domain.AlertRuleRecord, error) {
	return domain.AlertRuleRecord{}, alerting.ErrManagementUnavailable
}

func (alertManagerFunc) ReplaceRule(context.Context, domain.AlertRuleID, int64, alerting.RuleInput, domain.UserID, domain.RequestID) (domain.AlertRuleRecord, error) {
	return domain.AlertRuleRecord{}, alerting.ErrManagementUnavailable
}

func (alertManagerFunc) DeleteRule(context.Context, domain.AlertRuleID, int64, domain.UserID, domain.RequestID) error {
	return alerting.ErrManagementUnavailable
}

func (staticClusterCollector) Snapshot(context.Context) (kubernetes.Snapshot, error) {
	return kubernetes.Snapshot{
		ObservedAt: time.Date(2026, 8, 16, 11, 0, 0, 0, time.UTC), ClusterID: "cluster-one", ClusterName: "Main cluster",
		Namespaces: []kubernetes.Namespace{{Name: "shop", Phase: "Active"}},
	}, nil
}

func TestDemoProductReadFlowRequiresSessionAndReturnsSyntheticData(t *testing.T) {
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
	})

	unauthenticated := serve(handler, makeRequest(http.MethodGet, "/api/v1/sources", "198.51.100.10:4000"))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated sources status = %d", unauthenticated.Code)
	}

	sources := authenticatedProductRequest(handler, "/api/v1/sources")
	if sources.Code != http.StatusOK {
		t.Fatalf("sources status = %d, body = %q", sources.Code, sources.Body.String())
	}
	var sourceItems []sourceResponse
	if err := json.Unmarshal(sources.Body.Bytes(), &sourceItems); err != nil || len(sourceItems) != 1 ||
		!sourceItems[0].Synthetic || sourceItems[0].Resources != 6 || sourceItems[0].Health.Unhealthy != 1 {
		t.Fatalf("sources = %#v, %v", sourceItems, err)
	}

	environments := authenticatedProductRequest(handler, "/api/v1/environments?limit=10")
	var environmentPage environmentPageResponse
	if environments.Code != http.StatusOK || json.Unmarshal(environments.Body.Bytes(), &environmentPage) != nil ||
		len(environmentPage.Items) != 1 || environmentPage.Items[0].Availability != "available" || !environmentPage.Items[0].Synthetic {
		t.Fatalf("environments = (%d, %#v, %q)", environments.Code, environmentPage, environments.Body.String())
	}

	first := authenticatedProductRequest(handler, "/api/v1/resources?limit=2")
	var firstPage resourcePageResponse
	if first.Code != http.StatusOK || json.Unmarshal(first.Body.Bytes(), &firstPage) != nil ||
		len(firstPage.Items) != 2 || !firstPage.Page.HasMore || firstPage.Page.NextCursor == "" {
		t.Fatalf("first resource page = (%d, %#v, %q)", first.Code, firstPage, first.Body.String())
	}

	second := authenticatedProductRequest(handler, "/api/v1/resources?limit=2&cursor="+url.QueryEscape(firstPage.Page.NextCursor))
	var secondPage resourcePageResponse
	if second.Code != http.StatusOK || json.Unmarshal(second.Body.Bytes(), &secondPage) != nil || len(secondPage.Items) != 2 ||
		secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatalf("second resource page = (%d, %#v)", second.Code, secondPage)
	}

	detail := authenticatedProductRequest(handler, "/api/v1/resources/"+string(firstPage.Items[0].ID))
	var resource resourceResponse
	if detail.Code != http.StatusOK || json.Unmarshal(detail.Body.Bytes(), &resource) != nil || resource.Health == "" || !resource.Synthetic {
		t.Fatalf("resource detail = (%d, %#v)", detail.Code, resource)
	}
	health := authenticatedProductRequest(handler, "/api/v1/resources/"+string(resource.ID)+"/health")
	var snapshot healthResponse
	if health.Code != http.StatusOK || json.Unmarshal(health.Body.Bytes(), &snapshot) != nil || snapshot.ResourceID != resource.ID ||
		len(snapshot.Reasons) != 1 || !snapshot.Synthetic {
		t.Fatalf("resource health = (%d, %#v)", health.Code, snapshot)
	}
}

func TestDemoResourceQueryRejectsUnknownAndReboundCursor(t *testing.T) {
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
	})
	unknown := authenticatedProductRequest(handler, "/api/v1/resources?secret=value")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown query status = %d", unknown.Code)
	}
	first := authenticatedProductRequest(handler, "/api/v1/resources?limit=1")
	var page resourcePageResponse
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	rebound := authenticatedProductRequest(handler, "/api/v1/resources?limit=1&search=api&cursor="+url.QueryEscape(page.Page.NextCursor))
	if rebound.Code != http.StatusBadRequest {
		t.Fatalf("rebound cursor status = %d", rebound.Code)
	}
}

func TestProductRoutesSelectOneProviderAndListAllSources(t *testing.T) {
	cluster, err := kubernetes.New("source_cluster", "Main cluster", staticClusterCollector{})
	if err != nil {
		t.Fatalf("cluster provider: %v", err)
	}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
		options.ProductProviders = []ProductProvider{demo.MustNew(), cluster}
	})

	sources := authenticatedProductRequest(handler, "/api/v1/sources")
	var sourceItems []sourceResponse
	if sources.Code != http.StatusOK || json.Unmarshal(sources.Body.Bytes(), &sourceItems) != nil || len(sourceItems) != 2 || sourceItems[1].ID != "source_cluster" || sourceItems[1].Synthetic {
		t.Fatalf("sources = (%d, %#v, %q)", sources.Code, sourceItems, sources.Body.String())
	}
	resources := authenticatedProductRequest(handler, "/api/v1/resources?source_id=source_cluster&limit=10")
	var page resourcePageResponse
	if resources.Code != http.StatusOK || json.Unmarshal(resources.Body.Bytes(), &page) != nil || len(page.Items) != 2 || page.Items[0].Provider != "kubernetes" || page.Items[0].Synthetic {
		t.Fatalf("cluster resources = (%d, %#v, %q)", resources.Code, page, resources.Body.String())
	}
	detail := authenticatedProductRequest(handler, "/api/v1/resources/"+string(page.Items[0].ID)+"?source_id=source_cluster")
	if detail.Code != http.StatusOK {
		t.Fatalf("cluster detail = (%d, %q)", detail.Code, detail.Body.String())
	}
	unsupported := authenticatedProductRequest(handler, "/api/v1/logs?source_id=source_cluster&limit=10")
	var unsupportedLogs logPageResponse
	if unsupported.Code != http.StatusOK || json.Unmarshal(unsupported.Body.Bytes(), &unsupportedLogs) != nil ||
		unsupportedLogs.Availability != string(domain.AvailabilityUnsupported) || unsupportedLogs.ReasonCode != "kubernetes.log.not_implemented" {
		t.Fatalf("cluster logs = (%d, %#v)", unsupported.Code, unsupportedLogs)
	}
	unknown := authenticatedProductRequest(handler, "/api/v1/resources?source_id=source_missing")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown source status = %d", unknown.Code)
	}
}

func TestDemoObservabilityRoutesReturnBoundedSyntheticData(t *testing.T) {
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
	})

	topology := authenticatedProductRequest(handler, "/api/v1/topology?limit=10")
	var relationships relationshipPageResponse
	if topology.Code != http.StatusOK || json.Unmarshal(topology.Body.Bytes(), &relationships) != nil ||
		len(relationships.Items) != 5 || !relationships.Items[0].Synthetic {
		t.Fatalf("topology = (%d, %#v, %q)", topology.Code, relationships, topology.Body.String())
	}

	logs := authenticatedProductRequest(handler, "/api/v1/logs?limit=10")
	var logPage logPageResponse
	if logs.Code != http.StatusOK || json.Unmarshal(logs.Body.Bytes(), &logPage) != nil ||
		len(logPage.Items) != 6 || !logPage.Items[0].Synthetic {
		t.Fatalf("logs = (%d, %#v, %q)", logs.Code, logPage, logs.Body.String())
	}

	metrics := authenticatedProductRequest(handler, "/api/v1/metrics/current")
	var metricSet metricResponseSet
	if metrics.Code != http.StatusOK || json.Unmarshal(metrics.Body.Bytes(), &metricSet) != nil ||
		len(metricSet.Items) != 18 || !metricSet.Items[0].Synthetic {
		t.Fatalf("metrics = (%d, %#v, %q)", metrics.Code, metricSet, metrics.Body.String())
	}

	history := authenticatedProductRequest(handler, "/api/v1/metrics/history?minutes=60&limit=25")
	var historySet metricHistoryResponse
	if history.Code != http.StatusOK || json.Unmarshal(history.Body.Bytes(), &historySet) != nil ||
		len(historySet.Items) != 25 || historySet.Completeness != "partial" || !historySet.Items[0].Synthetic {
		t.Fatalf("metric history = (%d, %#v, %q)", history.Code, historySet, history.Body.String())
	}

	events := authenticatedProductRequest(handler, "/api/v1/events?limit=10")
	var eventPage eventPageResponse
	if events.Code != http.StatusOK || json.Unmarshal(events.Body.Bytes(), &eventPage) != nil ||
		len(eventPage.Items) != 6 || !eventPage.Items[0].Synthetic {
		t.Fatalf("events = (%d, %#v, %q)", events.Code, eventPage, events.Body.String())
	}

	alerts := authenticatedProductRequest(handler, "/api/v1/alerts")
	var alertPage alertPageResponse
	if alerts.Code != http.StatusOK || json.Unmarshal(alerts.Body.Bytes(), &alertPage) != nil ||
		len(alertPage.Items) != 1 || alertPage.Items[0].State != "firing" || !alertPage.Items[0].Synthetic {
		t.Fatalf("alerts = (%d, %#v, %q)", alerts.Code, alertPage, alerts.Body.String())
	}

	invalid := authenticatedProductRequest(handler, "/api/v1/logs?unknown=value")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unknown observability query = (%d, %q)", invalid.Code, invalid.Body.String())
	}
}

func TestProductRoutesPublishExactAllowedMethod(t *testing.T) {
	handler, _, _ := makeHandler(t, nil)
	for _, path := range []string{
		"/api/v1/sources", "/api/v1/environments", "/api/v1/resources", "/api/v1/topology", "/api/v1/logs", "/api/v1/metrics/current", "/api/v1/metrics/history", "/api/v1/events", "/api/v1/alerts", "/api/v1/notifications", "/api/v1/audit",
		"/api/v1/resources/r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"/api/v1/resources/r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA/health",
	} {
		request := makeRequest(http.MethodPost, path, "198.51.100.10:4000")
		response := serve(handler, request)
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
			t.Errorf("POST %s = %d, Allow %q", path, response.Code, response.Header().Get("Allow"))
		}
	}
}

func TestWriteProviderProblemMapsSafeCodes(t *testing.T) {
	tests := []struct {
		code domain.ProviderErrorCode
		want int
	}{
		{domain.ProviderErrorInvalidRequest, http.StatusBadRequest},
		{domain.ProviderErrorPermissionDenied, http.StatusForbidden},
		{domain.ProviderErrorRateLimited, http.StatusTooManyRequests},
		{domain.ProviderErrorUnavailable, http.StatusServiceUnavailable},
		{domain.ProviderErrorInternal, http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(string(test.code), func(t *testing.T) {
			retryable := test.code == domain.ProviderErrorUnavailable || test.code == domain.ProviderErrorRateLimited || test.code == domain.ProviderErrorInternal
			providerError, err := domain.NewProviderError("demo.inventory.read", test.code, retryable, errors.New("safe fixture cause"))
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			writeProviderProblem(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil), providerError)
			if recorder.Code != test.want || strings.Contains(recorder.Body.String(), "safe fixture cause") {
				t.Fatalf("response = (%d, %q), want %d", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}

func TestAuditRouteRequiresAdministratorAndReturnsBoundedPage(t *testing.T) {
	occurredAt := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	reader := auditReaderFunc(func(_ context.Context, request domain.PageRequest) (domain.AuditPage, error) {
		return domain.AuditPage{
			Entries: []domain.AuditEntry{{
				ID: "audit_login", OccurredAt: occurredAt,
				Actor:  domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_admin"},
				Action: "auth.login", Target: domain.AuditTarget{Kind: "auth.account", ID: "user_admin"},
				Outcome: domain.AuditSuccess, RequestID: "request_login", ReasonCode: "auth.login_succeeded",
			}},
			Page: domain.PageInfo{Limit: request.Limit},
		}, nil
	})
	role := auth.RoleAdministrator
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.AuditReader = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{Principal: auth.Principal{Role: role}}, nil
		}}
	})
	response := authenticatedProductRequest(handler, "/api/v1/audit?limit=10")
	var page auditPageResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil ||
		len(page.Items) != 1 || page.Items[0].Action != "auth.login" || page.Items[0].Actor.Kind != "user" || page.Page.Limit != 10 {
		t.Fatalf("audit response = (%d, %#v, %q)", response.Code, page, response.Body.String())
	}

	role = auth.RoleViewer
	denied := authenticatedProductRequest(handler, "/api/v1/audit?limit=10")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("viewer audit status = %d", denied.Code)
	}
}

func TestAlertRouteReadsDurableNonDemoHistory(t *testing.T) {
	cluster, err := kubernetes.New("source_cluster", "Main cluster", staticClusterCollector{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	reader := alertReaderFunc(func(_ context.Context, sourceID domain.SourceID, request domain.PageRequest) (domain.AlertPage, error) {
		if sourceID != "source_cluster" || request.Limit != 1 {
			t.Fatalf("alert read = (%q, %#v)", sourceID, request)
		}
		return domain.AlertPage{
			Alerts: []domain.Alert{{
				ID: "alert_cluster", RuleID: "rule_cluster", SourceID: sourceID,
				State: domain.AlertFiring, Severity: domain.SeverityCritical,
				EvidenceRange: domain.TimeRange{Start: now.Add(-2 * time.Minute), End: now.Add(-time.Minute)},
				StartedAt:     now.Add(-2 * time.Minute), UpdatedAt: now,
				SummaryCode: "alert.provider_connection_lost", Synthetic: false,
			}},
			Page: domain.PageInfo{Limit: request.Limit},
		}, nil
	})
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.ProductProviders = []ProductProvider{demo.MustNew(), cluster}
		options.AlertReader = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
	})
	response := authenticatedProductRequest(handler, "/api/v1/alerts?source_id=source_cluster&limit=1")
	var page alertPageResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil ||
		len(page.Items) != 1 || page.Items[0].ID != "alert_cluster" || page.Page.Limit != 1 {
		t.Fatalf("durable alert response = (%d, %#v, %q)", response.Code, page, response.Body.String())
	}
}

func TestNotificationRouteReturnsLinkedBoundedPage(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	sourceID := domain.SourceID("source_demo")
	alertID := domain.AlertID("alert_demo")
	reader := notificationReaderFunc(func(_ context.Context, request domain.PageRequest) (domain.NotificationPage, error) {
		if request.Limit != 1 {
			t.Fatalf("notification read = %#v", request)
		}
		return domain.NotificationPage{
			Notifications: []domain.Notification{{
				ID: "notification_demo", SourceID: &sourceID, AlertID: &alertID,
				Kind: "notification.alert_firing", Severity: domain.SeverityError,
				SummaryCode: "alert.resource_unhealthy", CreatedAt: now, Synthetic: true,
			}},
			Page: domain.PageInfo{Limit: request.Limit},
		}, nil
	})
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.NotificationReader = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
	})
	response := authenticatedProductRequest(handler, "/api/v1/notifications?limit=1")
	var page notificationPageResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil ||
		len(page.Items) != 1 || page.Items[0].ID != "notification_demo" || page.Items[0].AlertID == nil ||
		*page.Items[0].AlertID != alertID || page.Page.Limit != 1 {
		t.Fatalf("notification response = (%d, %#v, %q)", response.Code, page, response.Body.String())
	}
}

func TestNotificationSettingsExposeOnlyOperatorReferenceToAdministrator(t *testing.T) {
	settings, err := notification.NewRuntimeSettings(true, "webhook_primary")
	if err != nil {
		t.Fatal(err)
	}
	role := auth.RoleAdministrator
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.NotificationSettings = settings
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{Principal: auth.Principal{Role: role}}, nil
		}}
	})
	response := authenticatedProductRequest(handler, "/api/v1/settings/notifications")
	var status notificationSettingsResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &status) != nil ||
		status.Adapter != notification.WebhookAdapter || !status.Enabled ||
		status.DestinationReferenceID != "webhook_primary" ||
		strings.Contains(response.Body.String(), "url") || strings.Contains(response.Body.String(), "key") {
		t.Fatalf("notification settings = (%d, %#v, %q)", response.Code, status, response.Body.String())
	}

	role = auth.RoleViewer
	denied := authenticatedProductRequest(handler, "/api/v1/settings/notifications")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("viewer notification settings status = %d", denied.Code)
	}
}

func TestNotificationTestRequiresRecentProofAndQueuesDurableIntent(t *testing.T) {
	settings, err := notification.NewRuntimeSettings(true, "webhook_primary")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	tester := notificationTesterFunc(func(_ context.Context, userID domain.UserID, requestID domain.RequestID) (domain.Notification, error) {
		called = true
		if userID != "user_admin" || requestID != "req_test" {
			t.Fatalf("test identity = (%s, %s)", userID, requestID)
		}
		return domain.Notification{ID: "notification_test"}, nil
	})
	authentication := &testAuthentication{authorizeRecentFunc: func(_ context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, error) {
		if input.Purpose != auth.RecentAuthPurposeNotificationTest || input.SessionToken == "" ||
			input.CSRFProof != strings.Repeat("B", 43) || input.RecentProof != strings.Repeat("C", 43) {
			return auth.Authentication{}, auth.ErrAuthorizationDenied
		}
		return auth.Authentication{Principal: auth.Principal{UserID: "user_admin", Role: auth.RoleAdministrator}}, nil
	}}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = authentication
		options.NotificationSettings = settings
		options.NotificationTester = tester
	})
	path := "/api/v1/settings/notifications/test"
	missing := makeRequest(http.MethodPost, path, "198.51.100.10:4000")
	missing.Header.Set("Origin", "http://stackplore.test")
	missing.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	if response := serve(handler, missing); response.Code != http.StatusForbidden || called {
		t.Fatalf("missing recent proof response = %d, called = %v", response.Code, called)
	}
	request := makeRequest(http.MethodPost, path, "198.51.100.10:4000")
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, strings.Repeat("B", 43))
	request.Header.Set(recentAuthHeaderName, strings.Repeat("C", 43))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	response := serve(handler, request)
	var body notificationTestResponse
	if response.Code != http.StatusAccepted || json.Unmarshal(response.Body.Bytes(), &body) != nil || !called ||
		body.NotificationID != "notification_test" || body.DestinationReferenceID != "webhook_primary" || body.State != "queued" {
		t.Fatalf("test response = (%d, %#v, %q), called = %v", response.Code, body, response.Body.String(), called)
	}
}

func TestAlertAcknowledgementRequiresCSRFAndAdministrator(t *testing.T) {
	now := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	called := false
	manager := alertManagerFunc(func(_ context.Context, alertID domain.AlertID, userID domain.UserID, requestID domain.RequestID) (domain.Alert, error) {
		called = true
		if alertID != "alert_one" || userID != "user_admin" || requestID != "req_test" {
			t.Fatalf("ack identity = (%s, %s, %s)", alertID, userID, requestID)
		}
		acknowledgedAt := now
		return domain.Alert{
			ID: alertID, RuleID: "rule_one", SourceID: "source_demo", State: domain.AlertAcknowledged,
			Severity:      domain.SeverityWarning,
			EvidenceRange: domain.TimeRange{Start: now.Add(-time.Minute), End: now.Add(-time.Second)},
			StartedAt:     now.Add(-time.Minute), UpdatedAt: now, AcknowledgedAt: &acknowledgedAt,
			SummaryCode: "alert.resource_unhealthy.matched",
		}, nil
	})
	authentication := &testAuthentication{authorizeUnsafeFunc: func(_ context.Context, token, proof string) (auth.Authentication, error) {
		if token == "" || proof != strings.Repeat("B", 43) {
			return auth.Authentication{}, auth.ErrCSRFRejected
		}
		return auth.Authentication{
			Principal: auth.Principal{UserID: "user_admin", Role: auth.RoleAdministrator},
			Context:   auth.SessionContext{IdleExpiresAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), AbsoluteExpiresAt: time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)},
		}, nil
	}}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.AlertManager = manager
		options.Authentication = authentication
	})
	path := "/api/v1/alerts/alert_one/acknowledge"
	missingProof := makeRequest(http.MethodPost, path, "198.51.100.10:4000")
	missingProof.Header.Set("Origin", "http://stackplore.test")
	missingProof.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	if response := serve(handler, missingProof); response.Code != http.StatusForbidden || called {
		t.Fatalf("missing CSRF response = %d, called = %v", response.Code, called)
	}
	request := makeRequest(http.MethodPost, path, "198.51.100.10:4000")
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, strings.Repeat("B", 43))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	response := serve(handler, request)
	if response.Code != http.StatusOK || !called {
		t.Fatalf("ack response = %d, called = %v, body = %q", response.Code, called, response.Body.String())
	}
}

func authenticatedProductRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := makeRequest(http.MethodGet, path, "198.51.100.10:4000")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	return serve(handler, request)
}
