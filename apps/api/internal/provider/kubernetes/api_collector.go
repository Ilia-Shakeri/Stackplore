package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync/atomic"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	discoveryv1 "k8s.io/client-go/kubernetes/typed/discovery/v1"
	networkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

const (
	kubernetesAPIBase       = "https://kubernetes.default.svc"
	minimumAPIResponseBytes = 64 * 1024
	maximumAPIResponseBytes = 32 * 1024 * 1024
	maximumAPIItems         = 10_000
	maximumAPIPages         = 100
	maximumNamespaces       = 64
)

// Connection settings come from the operator, never from a product request.
type APICollectorOptions struct {
	Client          *http.Client
	BearerToken     string
	TokenReader     func(context.Context) (string, error)
	Namespaces      []string
	ClusterScoped   bool
	MaximumItems    int
	MaximumResponse int64
	Now             func() time.Time
}

type APICollector struct {
	client          *http.Client
	core            corev1.CoreV1Interface
	apps            appsv1.AppsV1Interface
	batch           batchv1.BatchV1Interface
	networking      networkingv1.NetworkingV1Interface
	discovery       discoveryv1.DiscoveryV1Interface
	metrics         metricsv1beta1.MetricsV1beta1Interface
	namespaces      []string
	clusterScoped   bool
	maximumItems    int
	maximumResponse int64
	now             func() time.Time
}

func NewAPICollector(options APICollectorOptions) (*APICollector, error) {
	if options.Client == nil || !validBearerToken(options.BearerToken) ||
		options.MaximumItems < 1 || options.MaximumItems > maximumAPIItems ||
		options.MaximumResponse < minimumAPIResponseBytes || options.MaximumResponse > maximumAPIResponseBytes ||
		len(options.Namespaces) < 1 || len(options.Namespaces) > maximumNamespaces {
		return nil, fmt.Errorf("Kubernetes API collector options are invalid")
	}
	namespaces := slices.Clone(options.Namespaces)
	slices.Sort(namespaces)
	for index, namespace := range namespaces {
		if len(validation.IsDNS1123Label(namespace)) != 0 || index > 0 && namespace == namespaces[index-1] {
			return nil, fmt.Errorf("Kubernetes namespace scope is invalid")
		}
	}
	client := *options.Client
	if client.Timeout <= 0 || client.Timeout > time.Minute {
		client.Timeout = 15 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return ErrUnavailable }
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	}
	if transport, ok := base.(*http.Transport); ok {
		clone := transport.Clone()
		clone.Proxy = nil
		clone.DisableCompression = true
		clone.ResponseHeaderTimeout = minPositiveDuration(clone.ResponseHeaderTimeout, client.Timeout)
		clone.MaxResponseHeaderBytes = 64 * 1024
		base = clone
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	client.Transport = &boundedTransport{base: base, tokens: &bearerSource{token: options.BearerToken, read: options.TokenReader, now: now, expiresAt: now().Add(time.Minute)}, namespaces: namespaces,
		clusterScoped: options.ClusterScoped, maximum: options.MaximumResponse}
	config := &rest.Config{Host: kubernetesAPIBase, Timeout: client.Timeout,
		ContentConfig: rest.ContentConfig{ContentType: "application/json", AcceptContentTypes: "application/json"},
		RateLimiter:   flowcontrol.NewTokenBucketRateLimiter(20, 20), UserAgent: "stackplore-observer",
		WarningHandlerWithContext: rest.NoWarnings{}}
	core, err := corev1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	apps, err := appsv1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	batch, err := batchv1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	networking, err := networkingv1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	discovery, err := discoveryv1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	metrics, err := metricsv1beta1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	return &APICollector{client: &client, core: core, apps: apps, batch: batch, networking: networking, discovery: discovery,
		metrics: metrics, namespaces: namespaces, clusterScoped: options.ClusterScoped,
		maximumItems: options.MaximumItems, maximumResponse: options.MaximumResponse, now: now}, nil
}

func (collector *APICollector) Close() error {
	if collector != nil && collector.client != nil {
		collector.client.CloseIdleConnections()
		if transport, ok := collector.client.Transport.(*boundedTransport); ok {
			transport.tokens.close()
		}
	}
	return nil
}

func minPositiveDuration(current, maximum time.Duration) time.Duration {
	if current <= 0 || current > maximum {
		return maximum
	}
	return current
}

func (collector *APICollector) valid(ctx context.Context) error {
	if collector == nil || collector.core == nil || ctx == nil {
		return ErrUnavailable
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func (collector *APICollector) permits(namespace string) bool {
	return slices.Contains(collector.namespaces, namespace)
}

type observationBudgetKey struct{}

func (collector *APICollector) observationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	budget := &atomic.Int64{}
	budget.Store(collector.maximumResponse)
	return context.WithValue(bounded, observationBudgetKey{}, budget), cancel
}

// A failed later page preserves earlier observations without permitting retirement.
func readSDKList[T any](ctx context.Context, namespace string, budget *int,
	fetch func(context.Context, metav1.ListOptions) ([]T, string, error), allowMissingUID ...bool) ([]T, error) {
	items := make([]T, 0)
	continuation := ""
	seen := make(map[string]struct{})
	identities := make(map[string]struct{})
	for page := 0; page < maximumAPIPages; page++ {
		if ctx.Err() != nil {
			return items, ctx.Err()
		}
		if *budget <= 0 {
			return items, ErrTruncated
		}
		limit := min(*budget, 500)
		pageContext, cancel := context.WithTimeout(ctx, 5*time.Second)
		values, next, err := fetch(pageContext, metav1.ListOptions{Limit: int64(limit), Continue: continuation})
		cancel()
		if err != nil {
			mapped := mapAPIError(err)
			if errors.Is(mapped, ErrPermissionDenied) || errors.Is(mapped, ErrUnsupported) {
				return nil, mapped
			}
			return items, mapped
		}
		if len(values) > limit {
			return items, ErrTruncated
		}
		for index := range values {
			metadata, err := meta.Accessor(&values[index])
			if err != nil || metadata.GetName() == "" || len(metadata.GetName()) > 253 ||
				metadata.GetUID() == "" && (len(allowMissingUID) == 0 || !allowMissingUID[0]) ||
				len(metadata.GetUID()) > 512 || metadata.GetNamespace() != namespace {
				return items, ErrUnavailable
			}
			key := metadata.GetNamespace() + "/" + metadata.GetName()
			if _, duplicate := identities[key]; duplicate {
				return items, ErrUnavailable
			}
			identities[key] = struct{}{}
		}
		items = append(items, values...)
		*budget -= len(values)
		if next == "" {
			return items, nil
		}
		if len(next) > 4096 {
			return items, ErrUnavailable
		}
		if _, repeated := seen[next]; repeated {
			return items, ErrUnavailable
		}
		seen[next] = struct{}{}
		continuation = next
	}
	return items, ErrTruncated
}

func mapAPIError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) || errors.Is(err, ErrPermissionDenied) {
		return ErrPermissionDenied
	}
	if apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
		return ErrUnsupported
	}
	if apierrors.IsConflict(err) {
		return ErrVersionConflict
	}
	if errors.Is(err, ErrTruncated) {
		return ErrTruncated
	}
	return ErrUnavailable
}

func observationStatus(at time.Time, err error, data bool) domain.ObservationStatus {
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
	if err == nil {
		return status
	}
	status.ReasonCode = "kubernetes.observation.unavailable"
	if errors.Is(err, ErrPermissionDenied) {
		status.State = domain.AvailabilityPermissionDenied
		status.ReasonCode = "kubernetes.observation.denied"
	} else if errors.Is(err, ErrUnsupported) {
		status.State = domain.AvailabilityUnsupported
		status.ReasonCode = "kubernetes.observation.unsupported"
	} else {
		status.State = domain.AvailabilityUnavailable
	}
	if data || errors.Is(err, ErrTruncated) {
		status.State = domain.AvailabilityAvailable
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "kubernetes.observation.partial"
	}
	return status
}

func combineStatuses(at time.Time, statuses []domain.ObservationStatus) domain.ObservationStatus {
	if len(statuses) == 0 {
		return observationStatus(at, ErrUnavailable, false)
	}
	result := statuses[0]
	result.ObservedAt = at
	available, complete := false, true
	for _, status := range statuses {
		available = available || status.AllowsData()
		complete = complete && status.State == domain.AvailabilityAvailable && status.Completeness == domain.CompletenessComplete
		if status.State != result.State {
			result.State = domain.AvailabilityUnavailable
			result.ReasonCode = "kubernetes.observation.unavailable"
		}
	}
	if complete {
		return observationStatus(at, nil, false)
	}
	if available {
		result.State = domain.AvailabilityAvailable
		result.Completeness = domain.CompletenessPartial
		result.ReasonCode = "kubernetes.observation.partial"
	}
	return result
}

var ErrTruncated = errors.New("Kubernetes observation limit reached")
