package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func testWriter(t *testing.T, round func(*http.Request) (*http.Response, error)) *SDKWriter {
	t.Helper()
	writer, err := NewSDKWriter(WriterOptions{Client: &http.Client{Transport: roundTripFunc(round), Timeout: time.Second}, BearerToken: "unit-test-observer-independent", Namespaces: []string{"shop"}, MaximumResponse: 64 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	return writer
}

func writerFixture(version, uid, receipt string) string {
	return `{"metadata":{"name":"api","namespace":"shop","uid":"` + uid + `","resourceVersion":"` + version + `","annotations":{"stackplore.io/operation-id":"` + receipt + `"}},"spec":{"replicas":3,"template":{"metadata":{"annotations":{"stackplore.io/restart-request":"` + receipt + `"}}}}}`
}

func writerOperation() WorkloadOperation {
	return WorkloadOperation{Kind: "Deployment", Namespace: "shop", Name: "api", ExpectedUID: "uid-api", ResourceVersion: "17", OperationID: "operation_receipt_one"}
}

func TestSDKWriterCASFixedFieldsAndScope(t *testing.T) {
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
		t.Run(kind, func(t *testing.T) {
			patches := 0
			writer := testWriter(t, func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					return jsonResponse(200, writerFixture("17", "uid-api", "")), nil
				}
				patches++
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatal(err)
				}
				var patch struct {
					Metadata struct {
						UID             string            `json:"uid"`
						ResourceVersion string            `json:"resourceVersion"`
						Annotations     map[string]string `json:"annotations"`
					} `json:"metadata"`
				}
				if json.Unmarshal(body, &patch) != nil || patch.Metadata.UID != "uid-api" || patch.Metadata.ResourceVersion != "17" || patch.Metadata.Annotations[operationReceiptAnnotation] != "operation_receipt_one" || strings.Contains(request.URL.Path, "/scale") || strings.Contains(string(body), "replicas") {
					t.Fatal("restart lost immutable target CAS or fixed body")
				}
				return jsonResponse(200, writerFixture("18", "uid-api", "operation_receipt_one")), nil
			})
			operation := writerOperation()
			operation.Kind = kind
			version, err := writer.RestartWorkload(context.Background(), operation)
			if err != nil || version != "18" || patches != 1 {
				t.Fatalf("restart version=%q patches=%d err=%v", version, patches, err)
			}
			operation.Namespace = "other"
			if _, err := writer.RestartWorkload(context.Background(), operation); !errors.Is(err, ErrPermissionDenied) || patches != 1 {
				t.Fatal("outside scope reached writer")
			}
		})
	}
}

func TestSDKWriterRejectsOldUIDVersionAndDaemonScaleBeforePATCH(t *testing.T) {
	for _, test := range []struct{ name, uid, version string }{{"uid", "replacement", "17"}, {"version", "uid-api", "18"}} {
		t.Run(test.name, func(t *testing.T) {
			patches := 0
			writer := testWriter(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPatch {
					patches++
				}
				return jsonResponse(200, writerFixture(test.version, test.uid, "")), nil
			})
			if _, err := writer.RestartWorkload(context.Background(), writerOperation()); !errors.Is(err, ErrVersionConflict) || patches != 0 {
				t.Fatal("stale target reached mutation")
			}
		})
	}
	calls := 0
	writer := testWriter(t, func(*http.Request) (*http.Response, error) { calls++; return nil, ErrUnavailable })
	op := writerOperation()
	op.Kind = "DaemonSet"
	replicas := 3
	op.RequestedReplicas = &replicas
	if _, err := writer.ScaleWorkload(context.Background(), op); !errors.Is(err, ErrUnsupported) || calls != 0 {
		t.Fatal("DaemonSet scale reached network")
	}
}

func TestSDKWriterLostReplyIsUncertainNeverRetriedAndReceiptReconciles(t *testing.T) {
	patches := 0
	accepted := false
	writer := testWriter(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch {
			patches++
			accepted = true
			return nil, io.ErrUnexpectedEOF
		}
		if accepted {
			return jsonResponse(200, writerFixture("18", "uid-api", "operation_receipt_one")), nil
		}
		return jsonResponse(200, writerFixture("17", "uid-api", "")), nil
	})
	op := writerOperation()
	if _, err := writer.RestartWorkload(context.Background(), op); !errors.Is(err, domain.ErrOperationOutcomeUncertain) || patches != 1 {
		t.Fatal("lost reply was retried or called definite failure")
	}
	version, found, err := writer.ReconcileWorkload(context.Background(), op)
	if err != nil || !found || version != "18" || patches != 1 {
		t.Fatal("own receipt failed read-only reconciliation")
	}
	op.OperationID = "operation_other"
	if _, found, err := writer.ReconcileWorkload(context.Background(), op); err != nil || found {
		t.Fatal("coincidental state claimed as own success")
	}
}

func TestSDKWriterUnauthorizedPATCHIsNeverRefreshedOrRetried(t *testing.T) {
	patches, reads := 0, 0
	writer := testWriter(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			return jsonResponse(200, writerFixture("17", "uid-api", "")), nil
		}
		patches++
		return jsonResponse(401, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Unauthorized","code":401}`), nil
	})
	writer.client.Transport.(*boundedTransport).tokens.read = func(context.Context) (string, error) { reads++; return "unit-test-refreshed", nil }
	_, err := writer.RestartWorkload(context.Background(), writerOperation())
	if !errors.Is(err, ErrPermissionDenied) || patches != 1 || reads != 0 {
		t.Fatal("write authorization triggered retry")
	}
}

func TestReadOnlyProviderAndCollectorHaveNoMutationPort(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, emptyListTransport)
	if _, ok := any(collector).(OperationCollector); ok {
		t.Fatal("observer has mutation interface")
	}
	provider := newFixtureProvider(t, collectorFunc(func(context.Context) (Snapshot, error) { return fixtureSnapshot(), nil }))
	if _, ok := any(provider).(domain.OperationProvider); ok {
		t.Fatal("read-only provider has mutation interface")
	}
	transport := collector.client.Transport.(*boundedTransport)
	if transport.permits(http.MethodPatch, "/apis/apps/v1/namespaces/shop/deployments/api") {
		t.Fatal("observer transport permits mutation")
	}
}
