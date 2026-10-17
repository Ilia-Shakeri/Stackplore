package kubernetes

import (
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

type DependencyReference struct {
	Kind domain.ResourceKind
	Name string
}

func controllerOwner(owners []metav1.OwnerReference) (string, string) {
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			return owner.Kind, string(owner.UID)
		}
	}
	return "", ""
}

func mapNode(item corev1.Node) Node {
	result := Node{UID: string(item.UID), Name: item.Name, Metadata: emptyKubernetesMetadata()}
	for _, condition := range item.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			result.Ready, result.ReadyKnown = condition.Status == corev1.ConditionTrue, condition.Status != corev1.ConditionUnknown
			result.Metadata.Conditions = append(result.Metadata.Conditions, mapCondition("Ready", condition.Status))
		}
	}
	return result
}

func mapPod(item corev1.Pod) Pod {
	ownerKind, ownerUID := controllerOwner(item.OwnerReferences)
	result := Pod{UID: string(item.UID), CreatedAt: item.CreationTimestamp.UTC().Truncate(time.Microsecond), Namespace: item.Namespace, Name: item.Name, Phase: string(item.Status.Phase),
		OwnerKind: ownerKind, OwnerUID: ownerUID, Labels: item.Labels, ContainerRestarts: make(map[string]uint64), DependsOn: parseDependencies(item.Annotations["stackplore.io/depends-on"])}
	result.Metadata, result.ClaimNames, result.ConfigMapNames, result.SecretNames = mapPodSpec(item.Spec)
	for _, condition := range item.Status.Conditions {
		if condition.Type == corev1.PodReady {
			result.Ready, result.ReadyKnown = condition.Status == corev1.ConditionTrue, condition.Status != corev1.ConditionUnknown
		}
		if mapped := mapCondition(string(condition.Type), condition.Status); mapped.Type != "" {
			result.Metadata.Conditions = append(result.Metadata.Conditions, mapped)
		}
	}
	statuses := append(slices.Clone(item.Status.ContainerStatuses), item.Status.InitContainerStatuses...)
	byName := make(map[string]corev1.ContainerStatus, len(statuses))
	for _, status := range statuses {
		byName[status.Name] = status
	}
	for index := range result.Metadata.Containers {
		container := &result.Metadata.Containers[index]
		result.Containers = append(result.Containers, container.Name)
		status, exists := byName[container.Name]
		if !exists {
			continue
		}
		container.Ready = status.Ready
		if status.RestartCount >= 0 {
			container.RestartCount = uint64(status.RestartCount)
		}
		result.ContainerRestarts[container.Name] = container.RestartCount
		switch {
		case status.State.Running != nil:
			container.State = domain.KubernetesContainerRunning
			container.ReasonCode = "kubernetes.container.running"
		case status.State.Waiting != nil:
			container.State = domain.KubernetesContainerWaiting
			container.ReasonCode = containerReason(status.State.Waiting.Reason, false)
		case status.State.Terminated != nil:
			container.State = domain.KubernetesContainerTerminated
			container.ReasonCode = containerReason(status.State.Terminated.Reason, true)
		}
	}
	return result
}

func containerReason(reason string, terminated bool) domain.ReasonCode {
	switch reason {
	case "CrashLoopBackOff":
		return "kubernetes.container.crash_loop"
	case "ErrImagePull", "ImagePullBackOff", "InvalidImageName", "RegistryUnavailable":
		return "kubernetes.container.image_pull"
	case "CreateContainerConfigError", "CreateContainerError", "RunContainerError":
		return "kubernetes.container.config_error"
	case "ContainerCreating", "PodInitializing":
		return "kubernetes.container.creating"
	case "OOMKilled":
		return "kubernetes.container.out_of_memory"
	case "Completed":
		return "kubernetes.container.completed"
	case "Error", "DeadlineExceeded":
		return "kubernetes.container.error"
	}
	if terminated {
		return "kubernetes.container.terminated"
	}
	return "kubernetes.container.waiting"
}

func mapPodSpec(spec corev1.PodSpec) (domain.KubernetesResourceMetadata, []string, []string, []string) {
	metadata := emptyKubernetesMetadata()
	var claims, configs, secrets []string
	containers := append(slices.Clone(spec.Containers), spec.InitContainers...)
	for _, container := range containers {
		entry := domain.KubernetesContainerMetadata{Name: container.Name, State: domain.KubernetesContainerUnknown}
		if quantity, exists := container.Resources.Requests[corev1.ResourceCPU]; exists {
			entry.CPURequestCores = safeCPUQuantity(quantity)
		}
		if quantity, exists := container.Resources.Limits[corev1.ResourceCPU]; exists {
			entry.CPULimitCores = safeCPUQuantity(quantity)
		}
		if quantity, exists := container.Resources.Requests[corev1.ResourceMemory]; exists {
			entry.MemoryRequestBytes = safeMemoryQuantity(quantity)
		}
		if quantity, exists := container.Resources.Limits[corev1.ResourceMemory]; exists {
			entry.MemoryLimitBytes = safeMemoryQuantity(quantity)
		}
		metadata.Containers = append(metadata.Containers, entry)
		for _, source := range container.EnvFrom {
			if source.ConfigMapRef != nil {
				configs = appendUnique(configs, source.ConfigMapRef.Name)
			}
			if source.SecretRef != nil {
				secrets = appendUnique(secrets, source.SecretRef.Name)
			}
		}
		for _, variable := range container.Env {
			if variable.ValueFrom == nil {
				continue
			}
			if variable.ValueFrom.ConfigMapKeyRef != nil {
				configs = appendUnique(configs, variable.ValueFrom.ConfigMapKeyRef.Name)
			}
			if variable.ValueFrom.SecretKeyRef != nil {
				secrets = appendUnique(secrets, variable.ValueFrom.SecretKeyRef.Name)
			}
		}
	}
	for _, volume := range spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			claims = appendUnique(claims, volume.PersistentVolumeClaim.ClaimName)
		}
		if volume.ConfigMap != nil {
			configs = appendUnique(configs, volume.ConfigMap.Name)
		}
		if volume.Secret != nil {
			secrets = appendUnique(secrets, volume.Secret.SecretName)
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.ConfigMap != nil {
					configs = appendUnique(configs, source.ConfigMap.Name)
				}
				if source.Secret != nil {
					secrets = appendUnique(secrets, source.Secret.Name)
				}
			}
		}
	}
	for _, reference := range spec.ImagePullSecrets {
		secrets = appendUnique(secrets, reference.Name)
	}
	return metadata, claims, configs, secrets
}

func safeCPUQuantity(quantity resource.Quantity) *float64 {
	value := quantity.AsApproximateFloat64()
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1e9 {
		return nil
	}
	return &value
}

func safeMemoryQuantity(quantity resource.Quantity) *uint64 {
	approximate := quantity.AsApproximateFloat64()
	if math.IsNaN(approximate) || math.IsInf(approximate, 0) || approximate < 0 || approximate > 1<<53-1 {
		return nil
	}
	value := uint64(quantity.Value())
	return &value
}

func mapCondition(kind string, status corev1.ConditionStatus) domain.KubernetesConditionMetadata {
	types := map[string]domain.KubernetesConditionType{"Ready": domain.KubernetesConditionReady, "ContainersReady": domain.KubernetesConditionContainersReady,
		"Initialized": domain.KubernetesConditionInitialized, "PodScheduled": domain.KubernetesConditionScheduled, "Available": domain.KubernetesConditionAvailable,
		"Progressing": domain.KubernetesConditionProgressing, "ReplicaFailure": domain.KubernetesConditionReplicaFailure, "Complete": domain.KubernetesConditionComplete, "Failed": domain.KubernetesConditionFailed}
	result := domain.KubernetesConditionMetadata{Type: types[kind], Status: domain.KubernetesConditionUnknown, ReasonCode: "kubernetes.condition.unknown"}
	if status == corev1.ConditionTrue {
		result.Status = domain.KubernetesConditionTrue
		result.ReasonCode = "kubernetes.condition.true"
	}
	if status == corev1.ConditionFalse {
		result.Status = domain.KubernetesConditionFalse
		result.ReasonCode = "kubernetes.condition.false"
	}
	return result
}

func baseWorkload(metadata metav1.ObjectMeta, kind string, desired, ready int32, known bool, spec corev1.PodSpec) Workload {
	ownerKind, ownerUID := controllerOwner(metadata.OwnerReferences)
	result := Workload{UID: string(metadata.UID), Version: metadata.ResourceVersion, Kind: kind, Namespace: metadata.Namespace, Name: metadata.Name,
		Desired: int(desired), Ready: int(ready), ReadyKnown: known, OwnerKind: ownerKind, OwnerUID: ownerUID, DependsOn: parseDependencies(metadata.Annotations["stackplore.io/depends-on"])}
	result.Metadata, result.ClaimNames, result.ConfigMapNames, result.SecretNames = mapPodSpec(spec)
	return result
}

func mapDeployment(item appsv1.Deployment) Workload {
	desired := int32(1)
	if item.Spec.Replicas != nil {
		desired = *item.Spec.Replicas
	}
	result := baseWorkload(item.ObjectMeta, "Deployment", desired, item.Status.ReadyReplicas, true, item.Spec.Template.Spec)
	for _, condition := range item.Status.Conditions {
		if mapped := mapCondition(string(condition.Type), condition.Status); mapped.Type != "" {
			result.Metadata.Conditions = append(result.Metadata.Conditions, mapped)
		}
	}
	return result
}
func mapStatefulSet(item appsv1.StatefulSet) Workload {
	desired := int32(1)
	if item.Spec.Replicas != nil {
		desired = *item.Spec.Replicas
	}
	result := baseWorkload(item.ObjectMeta, "StatefulSet", desired, item.Status.ReadyReplicas, true, item.Spec.Template.Spec)
	return result
}
func mapReplicaSet(item appsv1.ReplicaSet) Workload {
	desired := int32(1)
	if item.Spec.Replicas != nil {
		desired = *item.Spec.Replicas
	}
	result := baseWorkload(item.ObjectMeta, "ReplicaSet", desired, item.Status.ReadyReplicas, true, item.Spec.Template.Spec)
	for _, condition := range item.Status.Conditions {
		if mapped := mapCondition(string(condition.Type), condition.Status); mapped.Type != "" {
			result.Metadata.Conditions = append(result.Metadata.Conditions, mapped)
		}
	}
	return result
}
func mapDaemonSet(item appsv1.DaemonSet) Workload {
	return baseWorkload(item.ObjectMeta, "DaemonSet", item.Status.DesiredNumberScheduled, item.Status.NumberReady, true, item.Spec.Template.Spec)
}
func mapJob(item batchv1.Job) Workload {
	desired := int32(1)
	if item.Spec.Completions != nil {
		desired = *item.Spec.Completions
	}
	result := baseWorkload(item.ObjectMeta, "Job", desired, item.Status.Succeeded, true, item.Spec.Template.Spec)
	for _, condition := range item.Status.Conditions {
		if mapped := mapCondition(string(condition.Type), condition.Status); mapped.Type != "" {
			result.Metadata.Conditions = append(result.Metadata.Conditions, mapped)
		}
	}
	return result
}
func mapCronJob(item batchv1.CronJob) Workload {
	return baseWorkload(item.ObjectMeta, "CronJob", 0, 0, false, item.Spec.JobTemplate.Spec.Template.Spec)
}
func mapService(item corev1.Service) Service {
	return Service{UID: string(item.UID), Namespace: item.Namespace, Name: item.Name, Selector: item.Spec.Selector, DependsOn: parseDependencies(item.Annotations["stackplore.io/depends-on"])}
}
func mapClaim(item corev1.PersistentVolumeClaim) PersistentVolumeClaim {
	return PersistentVolumeClaim{UID: string(item.UID), Namespace: item.Namespace, Name: item.Name, Phase: string(item.Status.Phase)}
}
func mapIngress(item networkingv1.Ingress) Ingress {
	result := Ingress{UID: string(item.UID), Namespace: item.Namespace, Name: item.Name}
	if item.Spec.DefaultBackend != nil && item.Spec.DefaultBackend.Service != nil {
		result.ServiceNames = appendUnique(result.ServiceNames, item.Spec.DefaultBackend.Service.Name)
	}
	for _, rule := range item.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil {
					result.ServiceNames = appendUnique(result.ServiceNames, path.Backend.Service.Name)
				}
			}
		}
	}
	return result
}
func mapEndpointSlice(item discoveryv1.EndpointSlice) EndpointSlice {
	result := EndpointSlice{UID: string(item.UID), Namespace: item.Namespace, Name: item.Name, ServiceName: item.Labels["kubernetes.io/service-name"]}
	for _, endpoint := range item.Endpoints {
		if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" && (endpoint.TargetRef.Namespace == "" || endpoint.TargetRef.Namespace == item.Namespace) {
			result.PodUIDs = appendUnique(result.PodUIDs, string(endpoint.TargetRef.UID))
		}
	}
	return result
}

func parseDependencies(value string) []DependencyReference {
	if len(value) > 2048 {
		return nil
	}
	entries := strings.Split(value, ",")
	if len(entries) > 32 {
		return nil
	}
	result := make([]DependencyReference, 0, len(entries))
	for _, entry := range entries {
		kind, name, found := strings.Cut(strings.TrimSpace(entry), "/")
		if !found || len(validation.IsDNS1123Subdomain(name)) != 0 {
			continue
		}
		kind = strings.ToLower(kind)
		if kind == "configmap" {
			kind = "config_map"
		}
		if kind == "persistentvolumeclaim" {
			kind = "persistent_volume_claim"
		}
		if !slices.Contains([]string{"pod", "service", "deployment", "statefulset", "daemonset", "replicaset", "job", "cronjob", "ingress", "persistent_volume_claim", "config_map", "secret"}, kind) {
			continue
		}
		reference := DependencyReference{Kind: domain.ResourceKind("kubernetes." + kind), Name: name}
		if !slices.Contains(result, reference) {
			result = append(result, reference)
		}
	}
	return result
}

func materializeSafeReferences(snapshot *Snapshot) {
	configs, secrets := map[string]ObjectRef{}, map[string]ObjectRef{}
	allowed := func(namespace string, kind domain.ResourceKind) bool {
		for _, partition := range snapshot.Partitions {
			if partition.Kind == kind && partition.Scope == namespaceScope(namespace) {
				return partition.Status.AllowsData()
			}
		}
		return false
	}
	add := func(namespace string, configNames, secretNames []string) {
		for _, name := range configNames {
			if !allowed(namespace, "kubernetes.config_map") {
				continue
			}
			key := namespace + "/" + name
			configs[key] = ObjectRef{UID: "ref/configmap/" + key, Namespace: namespace, Name: name}
		}
		for _, name := range secretNames {
			if !allowed(namespace, "kubernetes.secret") {
				continue
			}
			key := namespace + "/" + name
			secrets[key] = ObjectRef{UID: "ref/secret/" + key, Namespace: namespace, Name: name}
		}
	}
	for _, pod := range snapshot.Pods {
		add(pod.Namespace, pod.ConfigMapNames, pod.SecretNames)
	}
	for _, workload := range snapshot.Workloads {
		add(workload.Namespace, workload.ConfigMapNames, workload.SecretNames)
	}
	snapshot.ConfigMaps = sortedObjectRefs(configs)
	snapshot.Secrets = sortedObjectRefs(secrets)
}

func sortedObjectRefs(values map[string]ObjectRef) []ObjectRef {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]ObjectRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func appendUnique(items []string, value string) []string {
	if value == "" || slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

func emptyKubernetesMetadata() domain.KubernetesResourceMetadata {
	return domain.KubernetesResourceMetadata{Containers: []domain.KubernetesContainerMetadata{}, Conditions: []domain.KubernetesConditionMetadata{}}
}
