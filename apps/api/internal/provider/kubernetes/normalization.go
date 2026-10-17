package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"sort"
	"time"
)

func (provider *Provider) observe(ctx context.Context) (normalized, Snapshot, error) {
	snapshot, err := provider.collector.Snapshot(ctx)
	if err != nil {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.read", err)
	}
	if snapshot.ObservedAt.Location() != time.UTC || snapshot.ObservedAt.Nanosecond()%int(time.Microsecond) != 0 || snapshot.ClusterID == "" || snapshot.ClusterName == "" {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.read", errors.New("invalid collector snapshot"))
	}
	status := snapshot.Status
	if status.State == "" {
		status = domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: snapshot.ObservedAt}
	}
	if status.Validate() != nil || status.ObservedAt != snapshot.ObservedAt {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.read", ErrUnavailable)
	}
	state := normalized{
		status: status, health: make(map[domain.ResourceID]domain.HealthSnapshot),
		workloadReplicas: make(map[domain.ResourceID]domain.WorkloadReplicaSnapshot),
	}
	refs := make(map[string]domain.ResourceRef)
	names := make(map[string]string)
	add := func(kind domain.ResourceKind, scope domain.Scope, nativeID, name string, health domain.HealthState, reason domain.ReasonCode, metadata ...domain.KubernetesResourceMetadata) error {
		if len(state.resources) >= domain.MaxInventoryCaptureItems {
			return ErrUnavailable
		}
		identity, err := domain.NewResourceIdentity(domain.ProviderKubernetes, provider.source.ID, provider.environment, kind, scope, domain.NativeResourceID(nativeID))
		if err != nil {
			return err
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			return err
		}
		key := resourceKey(string(kind), nativeID)
		if _, exists := refs[key]; exists {
			return fmt.Errorf("duplicate resource %s", key)
		}
		refs[key] = ref
		// Container names are unique within a pod, not within a namespace.
		// Container edges and reads use the immutable pod-derived resource key.
		if kind != "kubernetes.container" {
			lookupKey := nameKey(string(kind), scope, name)
			if existing, exists := names[lookupKey]; exists && existing != key {
				return fmt.Errorf("duplicate resource name %s", lookupKey)
			}
			names[lookupKey] = key
		}
		resourceStatus := status
		for _, partition := range snapshot.Partitions {
			if partition.Kind == kind && partition.Scope == scope {
				resourceStatus = partition.Status
				break
			}
		}
		if !resourceStatus.AllowsData() {
			return ErrPermissionDenied
		}
		resource := domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: name}, Status: resourceStatus}
		if len(metadata) > 0 {
			facts := metadata[0]
			if facts.Containers == nil {
				facts.Containers = []domain.KubernetesContainerMetadata{}
			}
			if facts.Conditions == nil {
				facts.Conditions = []domain.KubernetesConditionMetadata{}
			}
			resource.Metadata = &domain.ResourceMetadata{Kubernetes: &facts}
		}
		state.resources = append(state.resources, resource)
		healthSnapshot := domain.HealthSnapshot{ResourceID: ref.ID, State: health, Status: resourceStatus}
		if health != domain.HealthUnknown {
			evidenceState := domain.EvidenceNegative
			if health == domain.HealthHealthy {
				evidenceState = domain.EvidencePositive
			}
			healthSnapshot.Evidence = []domain.HealthEvidence{{
				ID: domain.EvidenceID("evidence_kubernetes_" + shortDigest(string(ref.ID))), Code: reason, SourceCode: reason,
				Status: evidenceState, ObservedAt: snapshot.ObservedAt,
			}}
		}
		state.health[ref.ID] = healthSnapshot
		return nil
	}
	global := domain.Scope{Kind: domain.ScopeGlobal}
	if snapshot.ClusterObserved {
		if err := add("kubernetes.cluster", global, snapshot.ClusterID, snapshot.ClusterName, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Namespaces {
		health, reason := namespaceHealth(item)
		nativeID := item.UID
		if nativeID == "" {
			nativeID = item.Name
		}
		if err := add("kubernetes.namespace", namespaceScope(item.Name), nativeID, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		refs[resourceKey("kubernetes.namespace", item.Name)] = refs[resourceKey("kubernetes.namespace", nativeID)]
	}
	for _, item := range snapshot.Nodes {
		health, reason := conditionHealth(item.ReadyKnown, item.Ready, "node")
		if err := add("kubernetes.node", global, item.UID, item.Name, health, reason, item.Metadata); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Workloads {
		kind, err := workloadResourceKind(item.Kind)
		if err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		health, reason := workloadHealth(item)
		if err := add(kind, namespaceScope(item.Namespace), item.UID, item.Name, health, reason, item.Metadata); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		if item.Version != "" {
			state.resources[len(state.resources)-1].Resource.Version = kubernetesOperationVersion(item.Version)
		}
		if item.ReadyKnown && item.Desired >= 0 && item.Ready >= 0 {
			ref := refs[resourceKey(string(kind), item.UID)]
			state.workloadReplicas[ref.ID] = domain.WorkloadReplicaSnapshot{
				Resource: ref, Desired: uint64(item.Desired), Ready: uint64(item.Ready), Status: state.resources[len(state.resources)-1].Status,
			}
		}
	}
	for _, item := range snapshot.Pods {
		health, reason := podHealth(item)
		if err := add("kubernetes.pod", namespaceScope(item.Namespace), item.UID, item.Name, health, reason, item.Metadata); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		for _, name := range item.Containers {
			containerMetadata := domain.KubernetesResourceMetadata{}
			containerState, containerReason := health, reason
			for _, facts := range item.Metadata.Containers {
				if facts.Name == name {
					containerMetadata.Containers = append(containerMetadata.Containers, facts)
					containerState, containerReason = kubernetesContainerHealth(facts)
				}
			}
			if err := add("kubernetes.container", namespaceScope(item.Namespace), item.UID+"/container/"+name, name, containerState, containerReason, containerMetadata); err != nil {
				return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
			}
		}
	}
	for _, item := range snapshot.Services {
		if err := add("kubernetes.service", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Ingresses {
		if err := add("kubernetes.ingress", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.EndpointSlices {
		if err := add("kubernetes.endpoint_slice", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Claims {
		health, reason := claimHealth(item)
		if err := add("kubernetes.persistent_volume_claim", namespaceScope(item.Namespace), item.UID, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.ConfigMaps {
		if err := add("kubernetes.config_map", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Secrets {
		if err := add("kubernetes.secret", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	if err := provider.buildRelationships(&state, refs, names, snapshot); err != nil {
		if !errors.Is(err, ErrTruncated) {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		state.status.State = domain.AvailabilityAvailable
		state.status.Completeness = domain.CompletenessPartial
		state.status.ReasonCode = "kubernetes.topology.truncated"
	}
	sort.Slice(state.resources, func(left, right int) bool {
		return state.resources[left].Resource.Ref.ID < state.resources[right].Resource.Ref.ID
	})
	sort.Slice(state.relationships, func(left, right int) bool { return state.relationships[left].ID < state.relationships[right].ID })
	return state, snapshot, nil
}

func (provider *Provider) buildRelationships(state *normalized, refs map[string]domain.ResourceRef, names map[string]string, snapshot Snapshot) error {
	counter := 0
	link := func(fromKey, toKey string, kind domain.RelationshipKind, provenance domain.RelationshipProvenance, certainty domain.EvidenceCertainty, confidence float64) error {
		from, fromOK := refs[fromKey]
		to, toOK := refs[toKey]
		if !fromOK || !toOK {
			return nil
		}
		if len(state.relationships) >= domain.MaxInventoryCaptureItems {
			return ErrTruncated
		}
		counter++
		evidence := domain.RelationshipEvidence{
			ID: domain.EvidenceID(fmt.Sprintf("evidence_kubernetes_relation_%d", counter)), Provider: domain.ProviderKubernetes,
			SourceID: provider.source.ID, Provenance: provenance, Certainty: certainty, Confidence: confidence, ObservedAt: snapshot.ObservedAt,
		}
		relationship, err := domain.NewRelationship(from.ID, to.ID, kind, domain.RelationshipSourceToTarget, []domain.RelationshipEvidence{evidence})
		if err == nil {
			state.relationships = append(state.relationships, relationship)
		}
		return err
	}
	clusterKey := resourceKey("kubernetes.cluster", snapshot.ClusterID)
	for _, item := range snapshot.Namespaces {
		if err := link(clusterKey, resourceKey("kubernetes.namespace", item.Name), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
	}
	for _, item := range snapshot.Workloads {
		kind, _ := workloadResourceKind(item.Kind)
		workloadKey := resourceKey(string(kind), item.UID)
		if err := link(resourceKey("kubernetes.namespace", item.Namespace), resourceKey(string(kind), item.UID), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
		if item.OwnerUID != "" {
			ownerKind, err := workloadResourceKind(item.OwnerKind)
			if err == nil {
				if err := link(resourceKey(string(ownerKind), item.OwnerUID), workloadKey, domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
					return err
				}
			}
		}
		for _, name := range item.ClaimNames {
			if err := link(workloadKey, findByName(names, "kubernetes.persistent_volume_claim", item.Namespace, name), domain.RelationshipMounts, domain.ProvenanceStorageReference, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, entry := range []struct {
			kind  string
			names []string
		}{{"kubernetes.config_map", item.ConfigMapNames}, {"kubernetes.secret", item.SecretNames}} {
			for _, name := range entry.names {
				if err := link(workloadKey, findByName(names, entry.kind, item.Namespace, name), domain.RelationshipReferences, domain.ProvenanceDeclaredDependency, domain.EvidenceExplicit, 1); err != nil {
					return err
				}
			}
		}
		for _, dependency := range item.DependsOn {
			if err := link(workloadKey, findByName(names, string(dependency.Kind), item.Namespace, dependency.Name), domain.RelationshipDependsOn, domain.ProvenanceAnnotation, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	for _, item := range snapshot.Pods {
		podKey := resourceKey("kubernetes.pod", item.UID)
		if err := link(resourceKey("kubernetes.namespace", item.Namespace), podKey, domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
		if item.OwnerUID != "" {
			ownerKind, err := workloadResourceKind(item.OwnerKind)
			if err == nil {
				if err := link(resourceKey(string(ownerKind), item.OwnerUID), podKey, domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
					return err
				}
			}
		}
		for _, dependency := range item.DependsOn {
			if err := link(podKey, findByName(names, string(dependency.Kind), item.Namespace, dependency.Name), domain.RelationshipDependsOn, domain.ProvenanceAnnotation, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.Containers {
			if err := link(podKey, resourceKey("kubernetes.container", item.UID+"/container/"+name), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.ClaimNames {
			if err := link(podKey, findByName(names, "kubernetes.persistent_volume_claim", item.Namespace, name), domain.RelationshipMounts, domain.ProvenanceStorageReference, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.ConfigMapNames {
			if err := link(podKey, findByName(names, "kubernetes.config_map", item.Namespace, name), domain.RelationshipReferences, domain.ProvenanceDeclaredDependency, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.SecretNames {
			if err := link(podKey, findByName(names, "kubernetes.secret", item.Namespace, name), domain.RelationshipReferences, domain.ProvenanceDeclaredDependency, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	for _, service := range snapshot.Services {
		serviceKey := resourceKey("kubernetes.service", service.UID)
		for _, dependency := range service.DependsOn {
			if err := link(serviceKey, findByName(names, string(dependency.Kind), service.Namespace, dependency.Name), domain.RelationshipDependsOn, domain.ProvenanceAnnotation, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, pod := range snapshot.Pods {
			if service.Namespace == pod.Namespace && selectorMatches(service.Selector, pod.Labels) {
				if err := link(serviceKey, resourceKey("kubernetes.pod", pod.UID), domain.RelationshipSelects, domain.ProvenanceSelector, domain.EvidenceExplicit, 1); err != nil {
					return err
				}
			}
		}
	}
	for _, ingress := range snapshot.Ingresses {
		for _, serviceName := range ingress.ServiceNames {
			if err := link(resourceKey("kubernetes.ingress", ingress.UID), findByName(names, "kubernetes.service", ingress.Namespace, serviceName), domain.RelationshipRoutesTo, domain.ProvenanceRoute, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	for _, endpoint := range snapshot.EndpointSlices {
		endpointKey := resourceKey("kubernetes.endpoint_slice", endpoint.UID)
		if err := link(findByName(names, "kubernetes.service", endpoint.Namespace, endpoint.ServiceName), endpointKey, domain.RelationshipRoutesTo, domain.ProvenanceEndpoint, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
		for _, podUID := range endpoint.PodUIDs {
			if err := link(endpointKey, resourceKey("kubernetes.pod", podUID), domain.RelationshipRoutesTo, domain.ProvenanceEndpoint, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	return nil
}
