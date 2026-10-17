package kubernetes

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestSDKMetricsRequireStablePodIdentityAndWholeLifetimeWindow(t *testing.T) {
	for _, test := range []struct {
		name, createdAt, metricUID, window, afterUID string
		afterCode, wantMetrics, wantChecks           int
		wantState                                    domain.AvailabilityState
	}{
		{"stable_without_metric_uid", "2026-08-16T07:00:00Z", "", "30s", "pod-one", 200, 1, 1, domain.AvailabilityAvailable},
		{"stable_matching_metric_uid", "2026-08-16T07:00:00Z", "pod-one", "30s", "pod-one", 200, 1, 1, domain.AvailabilityAvailable},
		{"cached_previous_uid", "2026-08-16T07:00:00Z", "old-pod", "30s", "pod-one", 200, 0, 0, domain.AvailabilityUnavailable},
		{"unknown_creation", "", "", "30s", "pod-one", 200, 0, 0, domain.AvailabilityUnavailable},
		{"cached_sample_before_creation", "2026-08-16T08:31:00Z", "", "30s", "pod-one", 200, 0, 0, domain.AvailabilityUnavailable},
		{"window_crosses_creation", "2026-08-16T08:29:45Z", "", "30s", "pod-one", 200, 0, 0, domain.AvailabilityUnavailable},
		{"raw_fractional_window_crosses_creation", "2026-08-16T08:29:45Z", "", "15.012s", "pod-one", 200, 0, 0, domain.AvailabilityUnavailable},
		{"recreated_during_read", "2026-08-16T07:00:00Z", "", "30s", "pod-two", 200, 0, 1, domain.AvailabilityUnavailable},
		{"deleted_after_read", "2026-08-16T07:00:00Z", "", "30s", "pod-one", 404, 0, 1, domain.AvailabilityUnavailable},
		{"permission_lost_after_read", "2026-08-16T07:00:00Z", "", "30s", "pod-one", 403, 0, 1, domain.AvailabilityPermissionDenied},
	} {
		t.Run(test.name, func(t *testing.T) {
			checks, lists, metricReads := 0, 0, 0
			collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/api/v1/namespaces/shop/pods":
					lists++
					return jsonResponse(http.StatusOK, `{"items":[`+metricIdentityPod("pod-one", test.createdAt)+`]}`), nil
				case "/apis/metrics.k8s.io/v1beta1/namespaces/shop/pods":
					metricReads++
					return jsonResponse(http.StatusOK, fmt.Sprintf(`{"items":[{"metadata":{"uid":%q,"name":"api","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":%q,"containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]}]}`, test.metricUID, test.window)), nil
				case "/api/v1/namespaces/shop/pods/api":
					checks++
					if test.afterCode != 200 {
						reason := "NotFound"
						if test.afterCode == 403 {
							reason = "Forbidden"
						}
						return jsonResponse(test.afterCode, fmt.Sprintf(`{"kind":"Status","reason":%q,"code":%d}`, reason, test.afterCode)), nil
					}
					return jsonResponse(http.StatusOK, metricIdentityPod(test.afterUID, test.createdAt)), nil
				default:
					t.Fatal("metric identity escaped bounded namespace and Pod reads")
					return nil, ErrUnavailable
				}
			})
			result, err := collector.CurrentPodMetrics(context.Background())
			if err != nil || len(result.Metrics) != test.wantMetrics || result.Status.State != test.wantState || checks != test.wantChecks || lists != 1 || metricReads != 1 {
				t.Fatalf("identity result metrics=%d state=%s checks=%d lists=%d metricReads=%d error=%v", len(result.Metrics), result.Status.State, checks, lists, metricReads, err)
			}
			if test.wantMetrics == 1 && result.Metrics[0].PodUID != "pod-one" {
				t.Fatal("verified immutable UID missing from normalized usage")
			}
		})
	}
}

func TestSDKMetricsIdentityAndMetricsHaveIndependentItemBudgets(t *testing.T) {
	requests := 0
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		requests++
		if response, matched := stableMetricIdentityResponse(request); matched {
			return response, nil
		}
		if request.URL.Path != "/apis/metrics.k8s.io/v1beta1/namespaces/shop/pods" || request.URL.Query().Get("limit") != "1" {
			t.Fatal("metric request lost its independent bounded item budget")
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]}]}`), nil
	}, func(options *APICollectorOptions) { options.MaximumItems = 1 })
	result, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(result.Metrics) != 1 || result.Status.Completeness != domain.CompletenessComplete || requests != 3 {
		t.Fatalf("bounded independent reads metrics=%d requests=%d error=%v", len(result.Metrics), requests, err)
	}
}

func TestSDKMetricsKeepVerifiedPodWhenAnotherSampleHasNoContainers(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v1/namespaces/shop/pods" {
			return jsonResponse(http.StatusOK, `{"items":[`+metricIdentityPod("pod-one", "2026-08-16T07:00:00Z")+`,{"metadata":{"uid":"pod-two","name":"worker","namespace":"shop","creationTimestamp":"2026-08-16T07:00:00Z"}}]}`), nil
		}
		if request.URL.Path == "/api/v1/namespaces/shop/pods/api" {
			return jsonResponse(http.StatusOK, metricIdentityPod("pod-one", "2026-08-16T07:00:00Z")), nil
		}
		if request.URL.Path != "/apis/metrics.k8s.io/v1beta1/namespaces/shop/pods" {
			t.Fatal("invalid usage caused an unrelated identity request")
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]},{"metadata":{"name":"worker","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[]}]}`), nil
	})
	result, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(result.Metrics) != 1 || result.Metrics[0].PodUID != "pod-one" || result.Namespaces["shop"].Completeness != domain.CompletenessPartial {
		t.Fatalf("safe partial identity evidence lost metrics=%d status=%#v error=%v", len(result.Metrics), result.Namespaces["shop"], err)
	}
}

func TestProviderRejectsUnboundUsageAndPreservesObservedConfiguredFacts(t *testing.T) {
	for _, test := range []struct {
		name        string
		change      func(*Snapshot, *PodMetric)
		wantSamples int
	}{
		{"stable", func(*Snapshot, *PodMetric) {}, 3},
		{"unknown_metric_uid", func(_ *Snapshot, metric *PodMetric) { metric.PodUID = "" }, 1},
		{"another_metric_uid", func(_ *Snapshot, metric *PodMetric) { metric.PodUID = "old-pod" }, 1},
		{"unknown_pod_creation", func(snapshot *Snapshot, _ *PodMetric) { snapshot.Pods[0].CreatedAt = time.Time{} }, 1},
		{"sample_before_creation", func(snapshot *Snapshot, metric *PodMetric) {
			snapshot.Pods[0].CreatedAt = metric.SampledAt.Add(time.Second)
		}, 1},
		{"window_crosses_creation", func(snapshot *Snapshot, metric *PodMetric) {
			snapshot.Pods[0].CreatedAt = metric.SampledAt.Add(-15 * time.Second)
		}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := fixtureSnapshot()
			snapshot.Pods[0].CreatedAt = snapshot.ObservedAt.Add(-time.Hour)
			snapshot.Pods[0].ContainerRestarts = map[string]uint64{"api": 2}
			metric := PodMetric{PodUID: snapshot.Pods[0].UID, Namespace: snapshot.Pods[0].Namespace, PodName: snapshot.Pods[0].Name, SampledAt: snapshot.ObservedAt.Add(-time.Minute), Window: 30 * time.Second, Containers: []ContainerMetric{{Name: "api", CPUCores: 0.25, MemoryBytes: 128 << 20}}}
			test.change(&snapshot, &metric)
			provider := newFixtureProvider(t, metricFixtureCollector{snapshot: snapshot, metrics: []PodMetric{metric}})
			resources, err := provider.ListResources(context.Background(), inventoryQuery())
			if err != nil {
				t.Fatal(err)
			}
			var pod domain.ResourceRef
			for _, resource := range resources.Resources {
				if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" {
					pod = resource.Resource.Ref
				}
			}
			result, err := provider.GetCurrentMetrics(context.Background(), domain.CurrentMetricQuery{Source: inventoryQuery().Source, Resources: []domain.ResourceRef{pod}, Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount}})
			if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != test.wantSamples {
				t.Fatalf("provider identity samples result=%#v error=%v", result, err)
			}
			if test.wantSamples == 1 && (result.Sets[0].Samples[0].Name != domain.MetricRestartCount || result.Sets[0].Samples[0].Value != 2 || result.Sets[0].Status.Completeness != domain.CompletenessPartial || result.Sets[0].Status.State != domain.AvailabilityStale) {
				t.Fatal("unbound usage erased trusted observed restart facts or claimed complete coverage")
			}
		})
	}
}

func TestSDKMixedPostReadPermissionDenialFencesAllUsage(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v1/namespaces/shop/pods":
			return jsonResponse(http.StatusOK, `{"items":[`+metricIdentityPod("pod-one", "2026-08-16T07:00:00Z")+`,{"metadata":{"uid":"pod-two","name":"worker","namespace":"shop","creationTimestamp":"2026-08-16T07:00:00Z"}}]}`), nil
		case "/api/v1/namespaces/shop/pods/api":
			return jsonResponse(http.StatusOK, metricIdentityPod("pod-one", "2026-08-16T07:00:00Z")), nil
		case "/api/v1/namespaces/shop/pods/worker":
			return jsonResponse(http.StatusForbidden, `{"kind":"Status","reason":"Forbidden","code":403}`), nil
		case "/apis/metrics.k8s.io/v1beta1/namespaces/shop/pods":
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]},{"metadata":{"name":"worker","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[{"name":"worker","usage":{"cpu":"100m","memory":"16Mi"}}]}]}`), nil
		default:
			t.Fatal("metric authority check escaped exact bounded paths")
			return nil, ErrUnavailable
		}
	})
	result, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(result.Metrics) != 0 || result.Status.State != domain.AvailabilityPermissionDenied || result.Namespaces["shop"].State != domain.AvailabilityPermissionDenied {
		t.Fatalf("mixed permission loss allowed usage metrics=%d status=%#v error=%v", len(result.Metrics), result.Status, err)
	}
}

type metricIdentityAuthorityCollector struct {
	metricFixtureCollector
	status          domain.AvailabilityState
	namespaceDenied bool
}

func (collector metricIdentityAuthorityCollector) CurrentPodMetrics(context.Context) (MetricObservation, error) {
	result := MetricObservation{Status: observationStatus(collector.snapshot.ObservedAt, nil, false), Metrics: collector.metrics}
	switch collector.status {
	case domain.AvailabilityPermissionDenied:
		result.Status = observationStatus(collector.snapshot.ObservedAt, ErrPermissionDenied, false)
	case domain.AvailabilityUnsupported:
		result.Status = observationStatus(collector.snapshot.ObservedAt, ErrUnsupported, false)
	case domain.AvailabilityUnavailable:
		result.Status = observationStatus(collector.snapshot.ObservedAt, ErrUnavailable, false)
	}
	if collector.namespaceDenied {
		result.Namespaces = map[string]domain.ObservationStatus{"shop": observationStatus(collector.snapshot.ObservedAt, ErrPermissionDenied, false)}
	}
	return result, nil
}

func TestProviderMissingUsageMarksMixedFactsStaleButConfiguredOnlyStaysAvailable(t *testing.T) {
	for _, availability := range []domain.AvailabilityState{domain.AvailabilityUnsupported, domain.AvailabilityUnavailable} {
		for _, usage := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_usage=%t", availability, usage), func(t *testing.T) {
				snapshot := fixtureSnapshot()
				snapshot.Pods[0].ContainerRestarts = map[string]uint64{"api": 2}
				provider := newFixtureProvider(t, metricIdentityAuthorityCollector{metricFixtureCollector: metricFixtureCollector{snapshot: snapshot}, status: availability})
				resources, err := provider.ListResources(context.Background(), inventoryQuery())
				if err != nil {
					t.Fatal(err)
				}
				var pod domain.ResourceRef
				for _, resource := range resources.Resources {
					if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" {
						pod = resource.Resource.Ref
					}
				}
				names := []domain.MetricName{domain.MetricRestartCount}
				wantState, wantCompleteness := domain.AvailabilityAvailable, domain.CompletenessComplete
				if usage {
					names = append(names, domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes)
					wantState, wantCompleteness = domain.AvailabilityStale, domain.CompletenessPartial
				}
				result, err := provider.GetCurrentMetrics(context.Background(), domain.CurrentMetricQuery{Source: inventoryQuery().Source, Resources: []domain.ResourceRef{pod}, Names: names})
				if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 1 || result.Sets[0].Status.State != wantState || result.Sets[0].Status.Completeness != wantCompleteness {
					t.Fatalf("missing usage hid stale state result=%#v error=%v", result, err)
				}
			})
		}
	}
}

func TestProviderKnownUsageDenialCannotHideBehindConfiguredFacts(t *testing.T) {
	for _, namespaceDenied := range []bool{false, true} {
		for _, names := range [][]domain.MetricName{{domain.MetricCPUUsageCores, domain.MetricRestartCount}, {domain.MetricRestartCount}} {
			t.Run(fmt.Sprintf("namespace=%t_usage=%t", namespaceDenied, len(names) == 2), func(t *testing.T) {
				snapshot := fixtureSnapshot()
				snapshot.Pods[0].CreatedAt = snapshot.ObservedAt.Add(-time.Hour)
				snapshot.Pods[0].ContainerRestarts = map[string]uint64{"api": 2}
				status := domain.AvailabilityPermissionDenied
				if namespaceDenied {
					status = domain.AvailabilityAvailable
				}
				provider := newFixtureProvider(t, metricIdentityAuthorityCollector{metricFixtureCollector: metricFixtureCollector{snapshot: snapshot}, status: status, namespaceDenied: namespaceDenied})
				resources, err := provider.ListResources(context.Background(), inventoryQuery())
				if err != nil {
					t.Fatal(err)
				}
				var pod domain.ResourceRef
				for _, resource := range resources.Resources {
					if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" {
						pod = resource.Resource.Ref
					}
				}
				result, err := provider.GetCurrentMetrics(context.Background(), domain.CurrentMetricQuery{Source: inventoryQuery().Source, Resources: []domain.ResourceRef{pod}, Names: names})
				if err != nil || len(result.Sets) != 0 || result.Status.State != domain.AvailabilityPermissionDenied {
					t.Fatalf("known denial hidden by configured facts sets=%d status=%#v error=%v", len(result.Sets), result.Status, err)
				}
			})
		}
	}
}

func metricIdentityPod(uid, createdAt string) string {
	creation := ""
	if createdAt != "" {
		creation = fmt.Sprintf(`,"creationTimestamp":%q`, createdAt)
	}
	return fmt.Sprintf(`{"metadata":{"uid":%q,"name":"api","namespace":"shop"%s}}`, uid, creation)
}
