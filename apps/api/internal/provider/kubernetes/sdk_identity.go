package kubernetes

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type observedPodIdentity struct {
	UID       string
	CreatedAt time.Time
}

// The caller's observation context owns the aggregate deadline and byte budget.
func (collector *APICollector) readPodIdentity(ctx context.Context, namespace, name string) (observedPodIdentity, error) {
	if !collector.permits(namespace) || !safeKubernetesPathSegment(name) {
		return observedPodIdentity{}, ErrPermissionDenied
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pod, err := collector.core.Pods(namespace).Get(bounded, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return observedPodIdentity{}, ErrUnavailable
		}
		return observedPodIdentity{}, mapAPIError(err)
	}
	if pod.Namespace != namespace || pod.Name != name || pod.UID == "" || len(pod.UID) > 512 {
		return observedPodIdentity{}, ErrUnavailable
	}
	return observedPodIdentity{UID: string(pod.UID), CreatedAt: pod.CreationTimestamp.UTC().Truncate(time.Microsecond)}, nil
}
