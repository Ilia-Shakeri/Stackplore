package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
)

type policyLogProvider struct {
	ProductProvider
	message   string
	lineLimit int
}

func (provider *policyLogProvider) ObservationTime() time.Time { return demo.ObservationTime() }
func (provider *policyLogProvider) ReadLogs(_ context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	provider.lineLimit = query.LineLimit
	return domain.LogPage{Status: explorationQueryStatus(), Page: domain.PageInfo{Limit: query.LineLimit}, Records: []domain.LogRecord{{
		ID: "log_policy", ResourceID: query.Targets[0].Resource.ID, Stream: domain.LogStreamStdout,
		Sequence: 1, Severity: domain.LogSeverityInfo, Timestamp: query.Range.Start.Add(time.Microsecond), Message: provider.message, Synthetic: true,
	}}}, nil
}

type policySettingsReader struct {
	record domain.ApplicationSettingsRecord
}

func (reader policySettingsReader) Get(context.Context) (domain.ApplicationSettingsRecord, error) {
	return reader.record, nil
}

func TestConfiguredLogsApplySameAdditivePolicyForReadAndStreamSnapshots(t *testing.T) {
	t.Parallel()
	policy, err := logsafe.NewPolicy([]string{"customer_id"})
	if err != nil {
		t.Fatal(err)
	}
	for _, settingsEnabled := range []bool{false, true} {
		provider := &policyLogProvider{ProductProvider: demo.MustNew(), message: `{"nested":{"customer\u005fid":"synth-personal-7"},"password":"synth-personal-8","message":"done"}`}
		reader := configuredLogReader{reader: provider, policy: policy}
		if settingsEnabled {
			values := domain.DefaultApplicationValues()
			values.LogLineLimit = 10
			reader.settings = policySettingsReader{record: domain.ApplicationSettingsRecord{Version: 1, UpdatedAt: demo.ObservationTime(), Values: values}}
		}
		wrapped := configuredLogStreamProvider{ProductProvider: provider, configuredLogReader: reader}
		data, err := buildStreamSnapshot(context.Background(), streamLogs, wrapped)
		if err != nil || bytes.Contains(data, []byte("synth-personal")) || !bytes.Contains(data, []byte("[redacted]")) || !bytes.Contains(data, []byte(`"redacted":true`)) {
			t.Fatalf("stream policy failed: %v", err)
		}
		if settingsEnabled && provider.lineLimit != 10 {
			t.Fatal("policy bypassed configured line limit")
		}
		if !observationTime(wrapped).Equal(demo.ObservationTime()) {
			t.Fatal("stream wrapper lost provider observation clock")
		}
	}
}

func TestConfiguredLogsPreserveByteBudgetAndImmutableProviderData(t *testing.T) {
	t.Parallel()
	provider := &policyLogProvider{ProductProvider: demo.MustNew(), message: "customer_id=x"}
	policy, _ := logsafe.NewPolicy([]string{"customer_id"})
	reader := configuredLogReader{reader: provider, policy: policy}
	inventory, err := provider.ListResources(context.Background(), domain.InventoryQuery{Source: providerRef(provider), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1}})
	if err != nil {
		t.Fatal(err)
	}
	query := domain.LogReadQuery{Source: providerRef(provider), Targets: []domain.LogTarget{{Resource: inventory.Resources[0].Resource.Ref}}, Range: domain.TimeRange{Start: demo.ObservationTime().Add(-time.Minute), End: demo.ObservationTime()}, LineLimit: 10, ByteLimit: len(provider.message)}
	result, err := reader.ReadLogs(context.Background(), query)
	if err != nil || result.ValidateFor(query) != nil || !result.Records[0].Redacted || !result.Records[0].Truncated || strings.Contains(result.Records[0].Message, "=x") || !utf8.ValidString(result.Records[0].Message) {
		t.Fatalf("bounded policy read failed: %v", err)
	}
	reader.policy = logsafe.Policy{}
	result, err = reader.ReadLogs(context.Background(), query)
	if err != nil || result.Records[0].Message != provider.message || result.Records[0].Redacted {
		t.Fatal("empty policy changed default provider data")
	}
}

func TestLogPageAndAuthenticatedSSEUseDeploymentRedactionFields(t *testing.T) {
	provider := &policyLogProvider{ProductProvider: demo.MustNew(), message: "customer_id=synth-personal-7 password=synth-personal-8 done"}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.ProductProviders = []ProductProvider{provider}
		options.LogRedactionFields = []string{"customer_id"}
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_policy"}}, nil
		}}
	})
	page := authenticatedProductRequest(handler, "/api/v1/logs")
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), "synth-personal") || !strings.Contains(page.Body.String(), "[redacted]") {
		t.Fatal("log page/export source bypassed configured policy")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := makeRequest(http.MethodGet, "/api/v1/streams/logs", "198.51.100.10:4000").WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	recorder := newStreamRecorder()
	done := make(chan struct{})
	go func() { handler.ServeHTTP(recorder, request); close(done) }()
	var body bytes.Buffer
	for !bytes.Contains(body.Bytes(), []byte("event: logs.snapshot.v1")) {
		select {
		case chunk := <-recorder.writes:
			body.Write(chunk)
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("log stream frame unavailable")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("log stream did not release request")
	}
	if bytes.Contains(body.Bytes(), []byte("synth-personal")) || !bytes.Contains(body.Bytes(), []byte("[redacted]")) {
		t.Fatal("SSE bypassed configured policy")
	}
}

func TestHTTPOptionsRejectInvalidRedactionFields(t *testing.T) {
	options := testOptions(&bytes.Buffer{})
	options.LogRedactionFields = []string{"customer_id", "CUSTOMER-ID"}
	if _, err := NewHandler(options); err != errInvalidOptions {
		t.Fatal("ambiguous redaction fields accepted")
	}
}
