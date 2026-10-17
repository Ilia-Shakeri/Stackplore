package demo

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"sort"
	"strconv"
	"time"
)

func seedObservability(provider *Provider) error {
	refs := make(map[string]domain.ResourceRef, len(provider.resources))
	for _, resource := range provider.resources {
		refs[string(resource.Resource.Ref.Identity.NativeID())] = resource.Resource.Ref
	}
	edges := []struct {
		from, to   string
		kind       domain.RelationshipKind
		provenance domain.RelationshipProvenance
	}{
		{"gateway", "frontend", domain.RelationshipRoutesTo, domain.ProvenanceRoute},
		{"gateway", "api", domain.RelationshipRoutesTo, domain.ProvenanceRoute},
		{"api", "checkout", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"api", "postgres", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"api", "redis", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"checkout", "postgres", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"checkout", "redis", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"worker", "postgres", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"worker", "redis", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"grafana", "prometheus", domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency},
		{"prometheus", "checkout", domain.RelationshipReferences, domain.ProvenanceAnnotation},
		{"prometheus", "gateway", domain.RelationshipReferences, domain.ProvenanceAnnotation},
		{"prometheus", "postgres", domain.RelationshipReferences, domain.ProvenanceAnnotation},
	}
	for _, edge := range edges {
		from, to := refs[edge.from].ID, refs[edge.to].ID
		evidence := domain.RelationshipEvidence{ID: domain.EvidenceID(factID("edge", string(from), string(to), string(edge.kind), provider.clock.Format(time.RFC3339Nano))), Provider: domain.ProviderDemo, SourceID: SourceID, Provenance: edge.provenance, Certainty: domain.EvidenceExplicit, Confidence: 1, ObservedAt: provider.clock, Synthetic: true}
		relationship, err := domain.NewRelationship(from, to, edge.kind, domain.RelationshipSourceToTarget, []domain.RelationshipEvidence{evidence})
		if err != nil {
			return err
		}
		provider.relationships = append(provider.relationships, relationship)
	}
	sort.Slice(provider.relationships, func(left, right int) bool { return provider.relationships[left].ID < provider.relationships[right].ID })
	for step := 0; step <= provider.step; step++ {
		clock := observedAt.Add(time.Duration(step) * scenarioTick)
		for index, resource := range provider.resources {
			ref := resource.Resource.Ref
			message, logSeverity, eventSeverity := fixtureMessage(string(ref.Identity.NativeID()), step)
			timestamp := clock.Add(-time.Duration(len(provider.resources)-index) * 30 * time.Second)
			provider.logs = append(provider.logs, domain.LogRecord{ID: domain.LogEntryID(factID("log", string(ref.ID), strconv.Itoa(step))), ResourceID: ref.ID, Stream: domain.LogStreamStdout, Sequence: uint64(step*len(provider.resources) + index + 1), Severity: logSeverity, Timestamp: timestamp, Message: message, Synthetic: true})
			environmentID, resourceID := ref.Identity.EnvironmentID, ref.ID
			provider.events = append(provider.events, domain.InfrastructureEvent{ID: domain.EventID(factID("event", string(ref.ID), strconv.Itoa(step))), Provider: domain.ProviderDemo, SourceID: SourceID, EnvironmentID: &environmentID, ResourceID: &resourceID, Kind: "demo.health.changed", Severity: eventSeverity, ReasonCode: "demo.health.observed", Summary: message, OccurredAt: timestamp, ObservedAt: clock, Synthetic: true})
		}
	}
	sort.Slice(provider.events, func(left, right int) bool {
		if provider.events[left].OccurredAt.Equal(provider.events[right].OccurredAt) {
			return provider.events[left].ID < provider.events[right].ID
		}
		return provider.events[left].OccurredAt.After(provider.events[right].OccurredAt)
	})
	environmentID, resourceID := EnvironmentID, refs["checkout"].ID
	alert := domain.Alert{ID: "alert_demo_checkout", RuleID: domain.DemoCheckoutHealthRuleID, SourceID: SourceID, EnvironmentID: &environmentID, ResourceID: &resourceID, State: domain.AlertFiring, Severity: domain.SeverityError, EvidenceRange: domain.TimeRange{Start: observedAt.Add(-5 * time.Minute), End: observedAt.Add(-time.Minute)}, StartedAt: observedAt.Add(-5 * time.Minute), UpdatedAt: provider.clock, SummaryCode: "demo.alert.checkout_unhealthy", Synthetic: true}
	if provider.step > 0 {
		resolvedAt := observedAt.Add(scenarioTick)
		alert.State, alert.ResolvedAt, alert.SummaryCode = domain.AlertResolved, &resolvedAt, "alert.resource_unhealthy.clear"
	}
	provider.alerts = []domain.Alert{alert}
	return nil
}

func fixtureMessage(nativeID string, step int) (string, domain.LogSeverity, domain.Severity) {
	if nativeID == "checkout" {
		if step == 0 {
			return "checkout request failed status=503 reason=connection_pool_exhausted", domain.LogSeverityError, domain.SeverityError
		}
		if step == 1 {
			return "checkout restarted; connection pool recovering", domain.LogSeverityWarning, domain.SeverityWarning
		}
		return "checkout readiness recovered status=200", domain.LogSeverityInfo, domain.SeverityInfo
	}
	if nativeID == "gateway" && step < 2 {
		return "checkout upstream latency above demo threshold", domain.LogSeverityWarning, domain.SeverityWarning
	}
	messages := map[string]string{"api": "request completed status=200 duration_ms=34", "gateway": "upstream latency recovered status=200", "postgres": "database checkpoint completed; connection limit healthy", "redis": "cache hit ratio stable; job queue available", "worker": "job completed queue=notifications", "frontend": "catalog rendered; checkout availability visible", "prometheus": "synthetic service metrics collected; checkout readiness observed", "grafana": "monitoring dashboard refreshed from local fixture metrics"}
	return messages[nativeID], domain.LogSeverityInfo, domain.SeverityInfo
}
