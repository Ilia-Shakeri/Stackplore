package domain

import (
	"context"
	"fmt"
	"time"
)

type SourceProfileID string

const (
	SourceProfileDemoFixture             SourceProfileID = "demo_fixture"
	SourceProfileDockerSocketDevelopment SourceProfileID = "docker_socket_dev"
	SourceProfileDockerRestrictedProxy   SourceProfileID = "docker_restricted_proxy"
	SourceProfileKubernetesInCluster     SourceProfileID = "kubernetes_incluster"
	SourceProfileKubernetesKubeconfig    SourceProfileID = "kubernetes_kubeconfig"
)

// SourceConfiguration stores only a known operator profile and safe source metadata.
type SourceConfiguration struct {
	Source    InfrastructureSource
	ProfileID SourceProfileID
	Enabled   bool
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (configuration SourceConfiguration) Validate() error {
	if configuration.Source.Validate() != nil || configuration.Version < 1 ||
		validateUTCTime("source created time", configuration.CreatedAt) != nil || validateUTCTime("source updated time", configuration.UpdatedAt) != nil ||
		!repositoryTimeSupported(configuration.CreatedAt) || !repositoryTimeSupported(configuration.UpdatedAt) ||
		configuration.CreatedAt.Location() != time.UTC || configuration.UpdatedAt.Location() != time.UTC ||
		configuration.CreatedAt.IsZero() || configuration.UpdatedAt.Before(configuration.CreatedAt) {
		return ErrRepositoryInvalidInput
	}
	expected := map[ProviderKind]SourceProfileID{
		ProviderDemo:       SourceProfileDemoFixture,
		ProviderDocker:     SourceProfileDockerSocketDevelopment,
		ProviderKubernetes: SourceProfileKubernetesInCluster,
	}
	profileAllowed := configuration.ProfileID == expected[configuration.Source.Provider] || configuration.Source.Provider == ProviderDocker && configuration.ProfileID == SourceProfileDockerRestrictedProxy || configuration.Source.Provider == ProviderKubernetes && configuration.ProfileID == SourceProfileKubernetesKubeconfig
	if !profileAllowed ||
		configuration.Source.Synthetic != (configuration.Source.Provider == ProviderDemo) {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type SourceConfigurationPage struct {
	Configurations []SourceConfiguration
	Page           PageInfo
}

func (page SourceConfigurationPage) ValidateFor(request PageRequest) error {
	if page.Page.ValidateFor(request) != nil || len(page.Configurations) > request.Limit {
		return ErrRepositoryInvalidInput
	}
	for index, configuration := range page.Configurations {
		if configuration.Validate() != nil || index > 0 && page.Configurations[index-1].Source.ID >= configuration.Source.ID {
			return ErrRepositoryInvalidInput
		}
	}
	return nil
}

type CaptureToken struct {
	Source               ProviderRef
	ConfigurationVersion int64
	Generation           int64
	ObservedAt           time.Time
}

func (token CaptureToken) Validate() error {
	if token.Source.Validate() != nil || token.ConfigurationVersion < 1 || token.Generation < 1 ||
		validateUTCTime("capture time", token.ObservedAt) != nil || !repositoryTimeSupported(token.ObservedAt) {
		return ErrRepositoryInvalidInput
	}
	return nil
}

// CaptureBatch is a short write unit belonging to one active collection generation.
type CaptureBatch struct {
	Token            CaptureToken
	Environments     []Environment
	Resources        []ResourceSnapshot
	Health           []HealthSnapshot
	WorkloadReplicas []WorkloadReplicaSnapshot
	Relationships    []Relationship
	Metrics          []MetricSample
	Events           []InfrastructureEvent
}

func (batch CaptureBatch) Validate() error {
	count := len(batch.Environments) + len(batch.Resources) + len(batch.Health) + len(batch.WorkloadReplicas) + len(batch.Relationships) + len(batch.Metrics) + len(batch.Events)
	if batch.Token.Validate() != nil || count < 1 || count > MaxRepositoryWriteBatch {
		return ErrRepositoryInvalidInput
	}
	synthetic := batch.Token.Source.Provider == ProviderDemo
	seen := make(map[string]struct{}, count)
	checkID := func(kind, id string) bool {
		key := kind + "\x00" + id
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
		return true
	}
	for _, environment := range batch.Environments {
		if environment.Validate() != nil || environment.SourceID != batch.Token.Source.SourceID ||
			environment.Provider != batch.Token.Source.Provider || environment.Synthetic != synthetic ||
			environment.Status.ObservedAt.After(batch.Token.ObservedAt) || !repositoryTimeSupported(environment.Status.ObservedAt) ||
			!checkID("environment", string(environment.ID)) {
			return ErrRepositoryInvalidInput
		}
	}
	for _, resource := range batch.Resources {
		if resource.Validate() != nil || !batch.Token.Source.Matches(resource.Resource.Ref) || resource.Resource.Synthetic != synthetic ||
			resource.Status.ObservedAt.After(batch.Token.ObservedAt) || !repositoryTimeSupported(resource.Status.ObservedAt) ||
			!checkID("resource", string(resource.Resource.Ref.ID)) {
			return ErrRepositoryInvalidInput
		}
	}
	for _, health := range batch.Health {
		if health.Validate() != nil || health.Status.ObservedAt.After(batch.Token.ObservedAt) ||
			!repositoryTimeSupported(health.Status.ObservedAt) || !checkID("health", string(health.ResourceID)) {
			return ErrRepositoryInvalidInput
		}
		for _, evidence := range health.Evidence {
			if evidence.Synthetic != synthetic || !repositoryTimeSupported(evidence.ObservedAt) {
				return ErrRepositoryInvalidInput
			}
		}
	}
	for _, relationship := range batch.Relationships {
		if relationship.Validate() != nil || relationship.ObservedAt.After(batch.Token.ObservedAt) ||
			!repositoryTimeSupported(relationship.ObservedAt) || !checkID("relationship", string(relationship.ID)) {
			return ErrRepositoryInvalidInput
		}
		for _, evidence := range relationship.Evidence {
			if evidence.Provider != batch.Token.Source.Provider || evidence.SourceID != batch.Token.Source.SourceID ||
				evidence.Synthetic != synthetic || !repositoryTimeSupported(evidence.ObservedAt) {
				return ErrRepositoryInvalidInput
			}
		}
	}
	for _, workload := range batch.WorkloadReplicas {
		query := ResourceDetailQuery{Source: batch.Token.Source, ResourceID: workload.Resource.ID}
		if workload.ValidateFor(query) != nil || workload.Desired > 1<<53-1 || workload.Ready > 1<<53-1 ||
			workload.Status.ObservedAt.After(batch.Token.ObservedAt) || !repositoryTimeSupported(workload.Status.ObservedAt) ||
			!checkID("workload", string(workload.Resource.ID)) {
			return ErrRepositoryInvalidInput
		}
	}
	if len(batch.Metrics) > 0 && ValidateMetricSampleWriteBatch(batch.Metrics) != nil {
		return ErrRepositoryInvalidInput
	}
	for _, sample := range batch.Metrics {
		if sample.Synthetic != synthetic || sample.SampledAt.After(batch.Token.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
	}
	if len(batch.Events) > 0 && ValidateInfrastructureEventWriteBatch(batch.Events) != nil {
		return ErrRepositoryInvalidInput
	}
	for _, event := range batch.Events {
		if event.SourceID != batch.Token.Source.SourceID || event.Provider != batch.Token.Source.Provider ||
			event.ObservedAt.After(batch.Token.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
	}
	return nil
}

// Complete applies only to full environment and resource inventory. Optional ports have separate capabilities.
type CaptureCompletion struct {
	Token        CaptureToken
	CollectedAt  time.Time
	Status       ObservationStatus
	Capabilities CapabilitySet
	Complete     bool
	Coverage     []InventoryCoverage
}

func (completion CaptureCompletion) Validate() error {
	if !completion.CollectedAt.IsZero() && (validateUTCTime("collection receipt time", completion.CollectedAt) != nil || !repositoryTimeSupported(completion.CollectedAt) || completion.CollectedAt.Before(completion.Token.ObservedAt)) {
		return ErrRepositoryInvalidInput
	}
	if ValidateInventoryCoverage(completion.Token.Source, completion.Coverage) != nil {
		return ErrRepositoryInvalidInput
	}
	for _, item := range completion.Coverage {
		if item.Status.ObservedAt.After(completion.Token.ObservedAt) || completion.Complete && !item.Complete() {
			return ErrRepositoryInvalidInput
		}
	}
	if completion.Token.Validate() != nil || completion.Status.Validate() != nil ||
		!repositoryTimeSupported(completion.Status.ObservedAt) || completion.Status.ObservedAt.After(completion.Token.ObservedAt) ||
		completion.Capabilities.ValidateFor(completion.Token.Source) != nil {
		return ErrRepositoryInvalidInput
	}
	if completion.Complete && (completion.Status.State != AvailabilityAvailable || completion.Status.Completeness != CompletenessComplete) {
		return fmt.Errorf("complete capture needs complete available inventory")
	}
	for _, capability := range completion.Capabilities.Capabilities {
		if capability.Status.ObservedAt.After(completion.Token.ObservedAt) || !repositoryTimeSupported(capability.Status.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
	}
	return nil
}

type SourceConfigurationReader interface {
	ListSourceConfigurations(context.Context, PageRequest) (SourceConfigurationPage, error)
	GetSourceConfiguration(context.Context, SourceID) (SourceConfiguration, error)
}

type CaptureRepository interface {
	SourceConfigurationReader
	EnsureSourceConfigurations(context.Context, []SourceConfiguration) error
	BeginCapture(context.Context, SourceID, int64, time.Time) (CaptureToken, error)
	SaveCapture(context.Context, CaptureBatch) error
	FinishCapture(context.Context, CaptureCompletion) error
	RetireUnseen(context.Context, CaptureToken, int) (int64, error)
}

type ObservationReader interface {
	InventoryProvider
	HealthProvider
	WorkloadReplicaProvider
	TopologyProvider
	CurrentMetricProvider
	MetricHistoryProvider
	EventHistoryProvider
	CapabilityProvider
	GetObservationStatus(context.Context, ProviderRef) (ObservationStatus, error)
}

type ObservationRetentionPolicy struct {
	MetricsBefore    time.Time
	HealthBefore     time.Time
	EventsBefore     time.Time
	DeliveriesBefore time.Time
}

type ObservationRetentionRepository interface {
	PruneObservationHistory(context.Context, ObservationRetentionPolicy, int) (int64, error)
}
