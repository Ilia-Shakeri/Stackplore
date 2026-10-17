package domain

import (
	"testing"
	"time"
)

func TestAcknowledgementAndDemoSeedBindDurableEpisode(t *testing.T) {
	at := fixtureTime()
	command := AlertAcknowledgement{AlertID: "alert_review", UserID: "user_admin", TransitionID: "transition_review", OccurredAt: at, Audit: AuditEntry{ID: "audit_review", OccurredAt: at, Actor: AuditActor{Kind: AuditActorUser, ID: "user_admin"}, Action: "alert.acknowledge", Target: AuditTarget{Kind: "alert.instance", ID: "alert_review"}, Outcome: AuditSuccess, RequestID: "request_review", ReasonCode: "alert.acknowledged"}}
	if err := command.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := command
	invalid.AlertID = ""
	requireInvalid(t, invalid.Validate())
	invalid = command
	invalid.Audit.Actor.ID = "user_other"
	requireInvalid(t, invalid.Validate())
	rule := reviewRuleRecord().Rule
	alert := validPageAlert("alert_seed", at)
	alert.SourceID = rule.SourceID
	alert.RuleID = rule.ID
	alert.Synthetic = true
	mutation := AlertMutation{Alert: alert, DedupeKey: "seed_review", Transition: &AlertTransition{ID: "transition_seed", AlertID: alert.ID, To: AlertFiring, OccurredAt: at, Actor: AuditActor{Kind: AuditActorSystem}, ReasonCode: "alert.firing", EvidenceRange: alert.EvidenceRange}}
	seed := DemoAlertSeed{FixtureVersion: "fixture_v1", Rule: rule, Mutation: mutation}
	if err := seed.Validate(); err != nil {
		t.Fatal(err)
	}
	seed.Mutation.Alert.Synthetic = false
	requireInvalid(t, seed.Validate())
	seed.Mutation.Alert.Synthetic = true
	seed.Rule.SourceID = "source_other"
	requireInvalid(t, seed.Validate())
}

func TestMetricHistoryTieOrderIsStableAcrossResourcesNamesAndIDs(t *testing.T) {
	a := MetricSample{ID: "metric_a", ResourceID: fixtureResourceRef(t, "a").ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores, Value: 1, SampledAt: fixtureTime(), Interval: time.Second}
	b := a
	b.ResourceID = fixtureResourceRef(t, "b").ID
	if a.ResourceID > b.ResourceID {
		a, b = b, a
	}
	if metricSampleOrder(a, b) >= 0 || metricSampleOrder(b, a) <= 0 {
		t.Fatal("resource tie ordering unstable")
	}
	b = a
	b.Name = MetricMemoryUsageBytes
	b.Unit = MetricUnitBytes
	if metricSampleOrder(a, b) >= 0 || metricSampleOrder(b, a) <= 0 {
		t.Fatal("metric tie ordering unstable")
	}
	b = a
	b.ID = "metric_z"
	if metricSampleOrder(a, b) >= 0 || metricSampleOrder(b, a) <= 0 || metricSampleOrder(a, a) != 0 {
		t.Fatal("identity tie ordering unstable")
	}
}

func TestInfrastructureEventsRejectUnsafeFieldsBeforeStorage(t *testing.T) {
	resource := fixtureResourceRef(t, "event").ID
	environment := EnvironmentID("env_primary")
	base := InfrastructureEvent{ID: "event_review", Provider: ProviderDemo, SourceID: "source_demo", EnvironmentID: &environment, ResourceID: &resource, Kind: "provider.event", Severity: SeverityInfo, ReasonCode: "provider.changed", Summary: "Resource changed", OccurredAt: fixtureTime(), ObservedAt: fixtureTime(), Synthetic: true}
	for name, change := range map[string]func(*InfrastructureEvent){
		"id": func(e *InfrastructureEvent) { e.ID = "" }, "provider": func(e *InfrastructureEvent) { e.Provider = "plain" }, "source": func(e *InfrastructureEvent) { e.SourceID = "" },
		"environment": func(e *InfrastructureEvent) { v := EnvironmentID(""); e.EnvironmentID = &v }, "resource": func(e *InfrastructureEvent) { v := ResourceID(""); e.ResourceID = &v },
		"kind": func(e *InfrastructureEvent) { e.Kind = "plain" }, "severity": func(e *InfrastructureEvent) { e.Severity = "plain" }, "reason": func(e *InfrastructureEvent) { e.ReasonCode = "plain" },
		"unsafe summary": func(e *InfrastructureEvent) { e.Summary = "\x00" }, "occurrence": func(e *InfrastructureEvent) { e.OccurredAt = time.Time{} }, "receipt": func(e *InfrastructureEvent) { e.ObservedAt = time.Time{} },
	} {
		t.Run(name, func(t *testing.T) { e := base; change(&e); requireInvalid(t, e.Validate()) })
	}
}

func TestResourceMetadataRejectsWrongProviderAndMixedPayload(t *testing.T) {
	docker := DockerResourceMetadata{State: "running", Health: "healthy", Ports: []DockerPortMetadata{}, Mounts: []DockerMountMetadata{}, Labels: []DockerLabelMetadata{}}
	kube := KubernetesResourceMetadata{Containers: []KubernetesContainerMetadata{{Name: "api", State: KubernetesContainerRunning}}, Conditions: []KubernetesConditionMetadata{{Type: KubernetesConditionReady, Status: KubernetesConditionTrue, ReasonCode: "kubernetes.condition.true"}}}
	metadata := ResourceMetadata{Docker: &docker}
	// Reviewed typed metadata is permitted; mixed provider payloads are not.
	if err := metadata.ValidateFor(ProviderDocker); err != nil {
		t.Fatal(err)
	}
	requireInvalid(t, metadata.ValidateFor(ProviderKubernetes))
	metadata = ResourceMetadata{Kubernetes: &kube}
	if err := metadata.ValidateFor(ProviderKubernetes); err != nil {
		t.Fatal(err)
	}
	metadata.Docker = &docker
	requireInvalid(t, metadata.ValidateFor(ProviderDocker))
	requireInvalid(t, (ResourceMetadata{}).ValidateFor(ProviderDemo))
}
