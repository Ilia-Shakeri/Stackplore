package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

type notificationLaneTester struct{ webhook, matrix int }

func (tester *notificationLaneTester) RequestTest(context.Context, domain.UserID, domain.RequestID) (domain.Notification, error) {
	tester.webhook++
	return domain.Notification{ID: "notification_webhook_test"}, nil
}
func (tester *notificationLaneTester) RequestMatrixTest(context.Context, domain.UserID, domain.RequestID) (domain.Notification, error) {
	tester.matrix++
	return domain.Notification{ID: "notification_matrix_test"}, nil
}

func TestNotificationAdapterQueryIsClosedBeforeGrantConsumption(t *testing.T) {
	for _, test := range []struct {
		query, lane string
		status      int
	}{
		{"", "webhook", 202}, {"?adapter=generic_webhook", "webhook", 202}, {"?adapter=matrix", "matrix", 202},
		{"?adapter=other", "", 400}, {"?adapter=", "", 400}, {"?other=matrix", "", 400},
		{"?adapter=matrix&adapter=generic_webhook", "", 400}, {"?adapter=matrix;", "", 400}, {"?adapter=%", "", 400},
	} {
		t.Run(test.query, func(t *testing.T) {
			settings, err := notification.NewCombinedRuntimeSettings(true, "webhook_primary", true, "matrix_primary")
			if err != nil {
				t.Fatal(err)
			}
			tester := &notificationLaneTester{}
			grants := 0
			handler, _, _ := makeHandler(t, func(options *Options) {
				options.NotificationSettings = settings
				options.NotificationTester = tester
				options.Authentication = &testAuthentication{authorizeRecentFunc: func(_ context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, error) {
					grants++
					if input.Purpose != auth.RecentAuthPurposeNotificationTest {
						t.Fatal("wrong grant purpose")
					}
					return auth.Authentication{Principal: auth.Principal{UserID: "user_admin", Role: auth.RoleAdministrator}}, nil
				}}
			})
			request := makeRequest(http.MethodPost, "/api/v1/settings/notifications/test"+test.query, "198.51.100.10:4000")
			request.Header.Set("Origin", "http://stackplore.test")
			request.Header.Set(csrfHeaderName, strings.Repeat("B", 43))
			request.Header.Set(recentAuthHeaderName, strings.Repeat("C", 43))
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
			response := serve(handler, request)
			if response.Code != test.status {
				t.Fatalf("status=%d want%d", response.Code, test.status)
			}
			if test.status == 400 {
				if grants != 0 || tester.webhook != 0 || tester.matrix != 0 {
					t.Fatal("invalid query consumed grant or queued delivery")
				}
				return
			}
			if grants != 1 || test.lane == "webhook" && (tester.webhook != 1 || tester.matrix != 0) || test.lane == "matrix" && (tester.matrix != 1 || tester.webhook != 0) || !strings.Contains(response.Body.String(), test.lane+"_primary") {
				t.Fatal("test escaped selected lane")
			}
		})
	}
}
