package demo

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const OperationsEnvironmentID domain.EnvironmentID = "env_demo_ops"
const scenarioTick = 5 * time.Minute

type fixtureResource struct {
	name, kind, nativeID string
	environment          domain.EnvironmentID
	scope                string
	profile              metricProfile
}

type metricProfile struct {
	cpu                             float64
	memoryMiB, restarts, networkMiB int
}

// NewAtStep selects a packaged scenario step without reading a runtime clock.
func NewAtStep(step int) (*Provider, error) {
	if step < 0 || step > 2 {
		return nil, fmt.Errorf("demo scenario step is invalid")
	}
	provider := &Provider{
		source: domain.InfrastructureSource{ID: SourceID, Provider: domain.ProviderDemo, Name: "Demo shop and operations", Synthetic: true},
		clock:  observedAt.Add(time.Duration(step) * scenarioTick), step: step,
		health: make(map[domain.ResourceID]domain.HealthSnapshot), profiles: make(map[domain.ResourceID]metricProfile),
	}
	status := provider.availableStatus()
	provider.environments = []domain.Environment{
		{ID: EnvironmentID, SourceID: SourceID, Provider: domain.ProviderDemo, Name: "Demo shop", Status: status, Synthetic: true},
		{ID: OperationsEnvironmentID, SourceID: SourceID, Provider: domain.ProviderDemo, Name: "Demo operations", Status: status, Synthetic: true},
	}
	sort.Slice(provider.environments, func(left, right int) bool { return provider.environments[left].ID < provider.environments[right].ID })
	definitions := []fixtureResource{
		{"API", "demo.service", "api", EnvironmentID, "demo-shop", metricProfile{0.42, 256, 0, 128}},
		{"Checkout", "demo.service", "checkout", EnvironmentID, "demo-shop", metricProfile{0.92, 384, 3, 96}},
		{"Gateway", "demo.gateway", "gateway", EnvironmentID, "demo-shop", metricProfile{0.18, 96, 0, 256}},
		{"PostgreSQL", "demo.database", "postgres", EnvironmentID, "demo-shop", metricProfile{0.35, 1024, 0, 192}},
		{"Redis", "demo.cache", "redis", EnvironmentID, "demo-shop", metricProfile{0.12, 128, 0, 128}},
		{"Worker", "demo.worker", "worker", EnvironmentID, "demo-shop", metricProfile{0.26, 192, 1, 64}},
		{"Frontend", "demo.frontend", "frontend", EnvironmentID, "demo-shop", metricProfile{0.08, 64, 0, 64}},
		{"Monitoring", "demo.monitoring", "prometheus", OperationsEnvironmentID, "demo-ops", metricProfile{0.20, 256, 0, 96}},
		{"Monitoring dashboard", "demo.dashboard", "grafana", OperationsEnvironmentID, "demo-ops", metricProfile{0.10, 128, 0, 32}},
	}
	for _, definition := range definitions {
		identity, err := domain.NewResourceIdentity(domain.ProviderDemo, SourceID, definition.environment, domain.ResourceKind(definition.kind), domain.Scope{Kind: domain.ScopeProject, Name: definition.scope}, domain.NativeResourceID(definition.nativeID))
		if err != nil {
			return nil, err
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			return nil, err
		}
		state := fixtureHealth(definition.nativeID, step)
		provider.resources = append(provider.resources, domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: definition.name, Synthetic: true}, Status: status, Health: state})
		provider.profiles[ref.ID] = definition.profile
		evidenceStatus, code := domain.EvidencePositive, domain.ReasonCode("demo.health.ready")
		if state == domain.HealthDegraded {
			evidenceStatus, code = domain.EvidenceUnknown, "demo.health.latency"
		}
		if state == domain.HealthUnhealthy {
			evidenceStatus, code = domain.EvidenceNegative, "demo.health.failure"
		}
		provider.health[ref.ID] = domain.HealthSnapshot{ResourceID: ref.ID, State: state, Status: status, Evidence: []domain.HealthEvidence{{ID: domain.EvidenceID(factID("health", string(ref.ID), provider.clock.Format(time.RFC3339Nano))), Code: code, SourceCode: code, Status: evidenceStatus, ObservedAt: provider.clock, Synthetic: true}}}
	}
	if err := seedObservability(provider); err != nil {
		return nil, err
	}
	if err := provider.validate(); err != nil {
		return nil, err
	}
	return provider, nil
}

func fixtureHealth(nativeID string, step int) domain.HealthState {
	if nativeID == "checkout" {
		if step == 0 {
			return domain.HealthUnhealthy
		}
		if step == 1 {
			return domain.HealthDegraded
		}
	}
	if nativeID == "gateway" && step < 2 {
		return domain.HealthDegraded
	}
	return domain.HealthHealthy
}

func factID(kind string, parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(append([]string{FixtureVersion, kind}, parts...), "\x00")))
	return "demo_" + kind + "_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func (provider *Provider) ScenarioStep() int { return provider.step }

func (provider *Provider) knownResource(ref domain.ResourceRef) bool {
	for _, resource := range provider.resources {
		if resource.Resource.Ref == ref {
			return true
		}
	}
	return false
}

func cloneHealth(snapshot domain.HealthSnapshot) domain.HealthSnapshot {
	snapshot.Evidence = slices.Clone(snapshot.Evidence)
	return snapshot
}
func cloneRelationships(items []domain.Relationship) []domain.Relationship {
	result := slices.Clone(items)
	for index := range result {
		result[index].Evidence = slices.Clone(result[index].Evidence)
	}
	return result
}
func cloneEvent(event domain.InfrastructureEvent) domain.InfrastructureEvent {
	if event.ResourceID != nil {
		value := *event.ResourceID
		event.ResourceID = &value
	}
	if event.EnvironmentID != nil {
		value := *event.EnvironmentID
		event.EnvironmentID = &value
	}
	return event
}
