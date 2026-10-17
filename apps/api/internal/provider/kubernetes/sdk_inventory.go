package kubernetes

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InventoryPartition describes only adapter-owned scope, never raw server errors.
type InventoryPartition struct {
	Kind   domain.ResourceKind
	Scope  domain.Scope
	Status domain.ObservationStatus
}

func (collector *APICollector) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := collector.valid(ctx); err != nil {
		return Snapshot{}, err
	}
	ctx, cancel := collector.observationContext(ctx)
	defer cancel()
	at := collector.now().UTC().Truncate(time.Microsecond)
	clusterID := collector.clusterID
	if clusterID == "" {
		clusterID = kubernetesAPIBase
	}
	snapshot := Snapshot{ObservedAt: at, ClusterID: clusterID, ClusterName: "Kubernetes", ContextName: "operator-profile"}
	budget := collector.maximumItems
	for _, namespace := range collector.namespaces {
		collector.collectNamespace(ctx, namespace, &budget, &snapshot)
	}
	collector.collectCluster(ctx, &budget, &snapshot)
	materializeSafeReferences(&snapshot)
	boundNormalizedSnapshot(&snapshot, collector.maximumItems)
	statuses := make([]domain.ObservationStatus, len(snapshot.Partitions))
	for index, partition := range snapshot.Partitions {
		statuses[index] = partition.Status
	}
	snapshot.Status = combineStatuses(at, statuses)
	if !collector.clusterScoped && snapshot.Status.AllowsData() && snapshot.Status.Completeness == domain.CompletenessComplete {
		snapshot.Status.Completeness = domain.CompletenessPartial
		snapshot.Status.ReasonCode = "kubernetes.profile.namespace_only"
	}
	if !snapshot.Status.AllowsData() {
		for _, status := range statuses {
			if status.State == domain.AvailabilityPermissionDenied {
				return Snapshot{}, ErrPermissionDenied
			}
		}
	}
	if ctx.Err() != nil && !snapshot.Status.AllowsData() {
		return Snapshot{}, ctx.Err()
	}
	return snapshot, nil
}

func (collector *APICollector) collectNamespace(ctx context.Context, namespace string, budget *int, snapshot *Snapshot) {
	add := func(kind domain.ResourceKind, err error, data bool) {
		snapshot.Partitions = append(snapshot.Partitions, InventoryPartition{Kind: kind, Scope: namespaceScope(namespace), Status: observationStatus(snapshot.ObservedAt, err, data)})
	}
	pods, podsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Pod, string, error) {
		page, err := collector.core.Pods(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range pods {
		mapped := mapPod(item)
		if mapped.Metadata.Validate() != nil {
			podsErr = ErrUnavailable
			continue
		}
		snapshot.Pods = append(snapshot.Pods, mapped)
	}
	add("kubernetes.pod", podsErr, len(pods) > 0)

	services, servicesErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Service, string, error) {
		page, err := collector.core.Services(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range services {
		snapshot.Services = append(snapshot.Services, mapService(item))
	}
	add("kubernetes.service", servicesErr, len(services) > 0)

	claims, claimsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.PersistentVolumeClaim, string, error) {
		page, err := collector.core.PersistentVolumeClaims(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range claims {
		snapshot.Claims = append(snapshot.Claims, mapClaim(item))
	}
	add("kubernetes.persistent_volume_claim", claimsErr, len(claims) > 0)

	deployments, deploymentsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]appsv1.Deployment, string, error) {
		page, err := collector.apps.Deployments(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range deployments {
		mapped := mapDeployment(item)
		if mapped.Metadata.Validate() != nil {
			deploymentsErr = ErrUnavailable
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, mapped)
	}
	add("kubernetes.deployment", deploymentsErr, len(deployments) > 0)

	statefulSets, statefulSetsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]appsv1.StatefulSet, string, error) {
		page, err := collector.apps.StatefulSets(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range statefulSets {
		mapped := mapStatefulSet(item)
		if mapped.Metadata.Validate() != nil {
			statefulSetsErr = ErrUnavailable
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, mapped)
	}
	add("kubernetes.statefulset", statefulSetsErr, len(statefulSets) > 0)

	daemonSets, daemonSetsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]appsv1.DaemonSet, string, error) {
		page, err := collector.apps.DaemonSets(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range daemonSets {
		mapped := mapDaemonSet(item)
		if mapped.Metadata.Validate() != nil {
			daemonSetsErr = ErrUnavailable
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, mapped)
	}
	add("kubernetes.daemonset", daemonSetsErr, len(daemonSets) > 0)

	replicaSets, replicaSetsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]appsv1.ReplicaSet, string, error) {
		page, err := collector.apps.ReplicaSets(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range replicaSets {
		mapped := mapReplicaSet(item)
		if mapped.Metadata.Validate() != nil {
			replicaSetsErr = ErrUnavailable
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, mapped)
	}
	add("kubernetes.replicaset", replicaSetsErr, len(replicaSets) > 0)

	jobs, jobsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]batchv1.Job, string, error) {
		page, err := collector.batch.Jobs(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range jobs {
		mapped := mapJob(item)
		if mapped.Metadata.Validate() != nil {
			jobsErr = ErrUnavailable
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, mapped)
	}
	add("kubernetes.job", jobsErr, len(jobs) > 0)

	cronJobs, cronJobsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]batchv1.CronJob, string, error) {
		page, err := collector.batch.CronJobs(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range cronJobs {
		mapped := mapCronJob(item)
		if mapped.Metadata.Validate() != nil {
			cronJobsErr = ErrUnavailable
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, mapped)
	}
	add("kubernetes.cronjob", cronJobsErr, len(cronJobs) > 0)

	ingresses, ingressesErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]networkingv1.Ingress, string, error) {
		page, err := collector.networking.Ingresses(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range ingresses {
		snapshot.Ingresses = append(snapshot.Ingresses, mapIngress(item))
	}
	add("kubernetes.ingress", ingressesErr, len(ingresses) > 0)

	endpoints, endpointsErr := readSDKList(ctx, namespace, budget, func(ctx context.Context, options metav1.ListOptions) ([]discoveryv1.EndpointSlice, string, error) {
		page, err := collector.discovery.EndpointSlices(namespace).List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range endpoints {
		snapshot.EndpointSlices = append(snapshot.EndpointSlices, mapEndpointSlice(item))
	}
	add("kubernetes.endpoint_slice", endpointsErr, len(endpoints) > 0)

	// Containers exist only in observed Pod specifications.
	add("kubernetes.container", podsErr, len(pods) > 0)
	// References can come from any Pod template, including a zero-replica workload.
	referenceErrors := errors.Join(podsErr, deploymentsErr, statefulSetsErr, daemonSetsErr, replicaSetsErr, jobsErr, cronJobsErr)
	// Parent provenance is not a separate cache partition. Do not keep old names
	// learned only through a parent whose read authority has since been revoked.
	if errors.Is(referenceErrors, ErrPermissionDenied) || errors.Is(referenceErrors, ErrUnsupported) {
		reason := error(ErrUnsupported)
		if errors.Is(referenceErrors, ErrPermissionDenied) {
			reason = ErrPermissionDenied
		}
		add("kubernetes.config_map", reason, false)
		add("kubernetes.secret", reason, false)
	} else {
		add("kubernetes.config_map", referenceErrors, namespaceHasReferences(snapshot, namespace, false))
		add("kubernetes.secret", referenceErrors, namespaceHasReferences(snapshot, namespace, true))
	}
}

func (collector *APICollector) collectCluster(ctx context.Context, budget *int, snapshot *Snapshot) {
	global := domain.Scope{Kind: domain.ScopeGlobal}
	add := func(kind domain.ResourceKind, scope domain.Scope, err error, data bool) {
		snapshot.Partitions = append(snapshot.Partitions, InventoryPartition{Kind: kind, Scope: scope, Status: observationStatus(snapshot.ObservedAt, err, data)})
	}
	if !collector.clusterScoped {
		// Omitted authority hides cached cluster-scoped records after a profile
		// reduction. An unavailable partition would incorrectly keep them stale.
		return
	}
	nodes, err := readSDKList(ctx, "", budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Node, string, error) {
		page, err := collector.core.Nodes().List(ctx, options)
		if err != nil {
			return nil, "", err
		}
		return page.Items, page.Continue, nil
	})
	for _, item := range nodes {
		snapshot.Nodes = append(snapshot.Nodes, mapNode(item))
	}
	add("kubernetes.node", global, err, len(nodes) > 0)
	observed := err == nil
	for _, namespace := range collector.namespaces {
		var item *corev1.Namespace
		namespaceErr := ctx.Err()
		if namespaceErr == nil && *budget <= 0 {
			namespaceErr = ErrTruncated
		}
		if namespaceErr == nil {
			item, namespaceErr = collector.core.Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		}
		if namespaceErr == nil {
			if item.UID == "" || item.Name != namespace {
				namespaceErr = ErrUnavailable
			} else {
				snapshot.Namespaces = append(snapshot.Namespaces, Namespace{UID: string(item.UID), Name: item.Name, Phase: string(item.Status.Phase)})
				*budget--
				observed = true
			}
		}
		add("kubernetes.namespace", namespaceScope(namespace), mapAPIError(namespaceErr), namespaceErr == nil)
	}
	snapshot.ClusterObserved = observed
	clusterErr := error(nil)
	if !observed {
		clusterErr = ErrUnavailable
	}
	add("kubernetes.cluster", global, clusterErr, observed)
}

func namespaceHasReferences(snapshot *Snapshot, namespace string, secret bool) bool {
	for _, pod := range snapshot.Pods {
		if pod.Namespace == namespace && (secret && len(pod.SecretNames) > 0 || !secret && len(pod.ConfigMapNames) > 0) {
			return true
		}
	}
	for _, workload := range snapshot.Workloads {
		if workload.Namespace == namespace && (secret && len(workload.SecretNames) > 0 || !secret && len(workload.ConfigMapNames) > 0) {
			return true
		}
	}
	return false
}

// SDK object pages and normalized resources have separate bounds because one Pod
// may produce many container and reference resources.
func boundNormalizedSnapshot(snapshot *Snapshot, maximum int) {
	remaining := maximum
	truncated := false
	take := func(length int) int {
		count := min(length, remaining)
		remaining -= count
		truncated = truncated || count < length
		return count
	}
	snapshot.Namespaces = snapshot.Namespaces[:take(len(snapshot.Namespaces))]
	snapshot.Nodes = snapshot.Nodes[:take(len(snapshot.Nodes))]
	snapshot.Workloads = snapshot.Workloads[:take(len(snapshot.Workloads))]
	pods := make([]Pod, 0, min(len(snapshot.Pods), remaining))
	for _, pod := range snapshot.Pods {
		if remaining == 0 {
			truncated = true
			break
		}
		remaining--
		pod.Containers = slices.Clone(pod.Containers[:take(len(pod.Containers))])
		pods = append(pods, pod)
	}
	snapshot.Pods = pods
	snapshot.Services = snapshot.Services[:take(len(snapshot.Services))]
	snapshot.Ingresses = snapshot.Ingresses[:take(len(snapshot.Ingresses))]
	snapshot.EndpointSlices = snapshot.EndpointSlices[:take(len(snapshot.EndpointSlices))]
	snapshot.Claims = snapshot.Claims[:take(len(snapshot.Claims))]
	snapshot.ConfigMaps = snapshot.ConfigMaps[:take(len(snapshot.ConfigMaps))]
	snapshot.Secrets = snapshot.Secrets[:take(len(snapshot.Secrets))]
	if snapshot.ClusterObserved {
		if remaining == 0 {
			snapshot.ClusterObserved = false
			truncated = true
		} else {
			remaining--
		}
	}
	if truncated {
		for index := range snapshot.Partitions {
			if snapshot.Partitions[index].Status.AllowsData() {
				snapshot.Partitions[index].Status = observationStatus(snapshot.ObservedAt, ErrTruncated, true)
			}
		}
	}
}
