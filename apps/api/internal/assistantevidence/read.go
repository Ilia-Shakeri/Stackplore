package assistantevidence

import (
	"context"
	"slices"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func readCapability(kind assistant.StepKind) (domain.CapabilityName, bool) {
	switch kind {
	case assistant.Inventory:
		return domain.CapabilityInventoryDetail, true
	case assistant.Health:
		return domain.CapabilityHealth, true
	case assistant.Metrics:
		return domain.CapabilityMetricHistory, true
	case assistant.Events:
		return domain.CapabilityEventHistory, true
	case assistant.Topology:
		return domain.CapabilityTopology, true
	case assistant.Logs:
		return domain.CapabilityLogRead, true
	default:
		return "", false
	}
}

// Read has no generation or admission capability. A live, admitted run is required.
// Every request performs one fixed read family, with no cursor-following loop.
func (reader *Reader) Read(ctx context.Context, request ReadRequest) (Evidence, error) {
	if reader == nil || ctx == nil {
		return Evidence{}, ErrInvalid
	}
	if request.Run.State != assistant.Running || !assistant.Timestamp(request.Run.Deadline) || !assistant.Timestamp(request.Run.LeaseUntil) || !request.Run.Deadline.After(reader.time()) || !request.Run.LeaseUntil.After(reader.time()) {
		return Evidence{}, ErrInvalid
	}
	ctx, stopLifetime := context.WithTimeout(ctx, min(request.Run.Deadline.Sub(reader.time()), request.Run.LeaseUntil.Sub(reader.time())))
	defer stopLifetime()
	work, cancel, err := reader.work(ctx)
	if err != nil {
		return Evidence{}, err
	}
	defer cancel()
	request.Scope = cloneScope(request.Scope)
	source, err := reader.validate(request.Actor, request.Run, request.Scope)
	if err != nil {
		return Evidence{}, err
	}
	capability, known := readCapability(request.Kind)
	if !known || request.Run.State != assistant.Running || request.Run.Version < 1 || request.Run.Fence < 1 || request.Run.ConfigurationVersion < 1 ||
		!assistant.Timestamp(request.Run.CreatedAt) || request.Run.CreatedAt.After(reader.time()) || !request.Run.Deadline.After(reader.time()) ||
		request.Run.Attempts < 1 || request.Run.Attempts > assistant.MaximumClaims || !assistant.Identifier(request.Run.LeaseOwner) || !assistant.Timestamp(request.Run.LeaseUntil) || !request.Run.LeaseUntil.After(reader.time()) || request.Run.LeaseUntil.After(request.Run.Deadline) || request.Run.UsageUncertain ||
		request.Destination != contextbuilder.Local && request.Destination != contextbuilder.Remote {
		return Evidence{}, ErrInvalid
	}
	if request.Kind == assistant.Logs && (!request.LogConsent || request.Destination != contextbuilder.Local || source.Logs == nil || request.Scope.Range.End.Sub(request.Scope.Range.Start) > contextbuilder.MaximumLogWindow) {
		return Evidence{}, ErrDenied
	}
	resources, err := reader.fence(work, request.Actor, request.Run, request.Scope, source, assistant.Inventory, false)
	if err != nil {
		return Evidence{}, err
	}
	capabilities, err := source.Read.GetCapabilities(work, request.Scope.Source)
	if err != nil {
		return Evidence{}, safeError(work, err)
	}
	if capabilities.ValidateFor(request.Scope.Source) != nil {
		return Evidence{}, ErrUnavailable
	}
	var status domain.ObservationStatus
	for _, item := range capabilities.Capabilities {
		if item.Name == capability {
			status = item.Status
		}
	}
	project := contextbuilder.Request{Scope: cloneScope(request.Scope), Destination: request.Destination, Status: status, RecordLimit: MaximumRecords, ByteLimit: contextbuilder.MaximumBytes}
	if status.State == domain.AvailabilityPermissionDenied {
		return Evidence{}, ErrDenied
	}
	if status.AllowsData() {
		if err := source.Live.Check(work, cloneScope(request.Scope), request.Kind, request.LogConsent); err != nil {
			return Evidence{}, safeError(work, err)
		}
		if err := reader.collect(work, source, request, resources, &project); err != nil {
			return Evidence{}, safeError(work, err)
		}
	}
	proofKind := assistant.Inventory
	if status.AllowsData() {
		proofKind = request.Kind
	}
	if _, err := reader.fence(work, request.Actor, request.Run, request.Scope, source, proofKind, request.LogConsent); err != nil {
		return Evidence{}, err
	}
	project.Scope.ObservedAt = reader.time()
	bundle, err := contextbuilder.Build(work, project)
	if err != nil {
		return Evidence{}, safeError(work, err)
	}
	if _, err := bundle.Items(); err != nil {
		return Evidence{}, safeError(work, err)
	}
	if _, err := reader.fence(work, request.Actor, request.Run, request.Scope, source, proofKind, request.LogConsent); err != nil {
		return Evidence{}, err
	}
	evidence := Evidence{reader: reader, actor: request.Actor, run: request.Run, scope: cloneScope(project.Scope), bundle: bundle, kind: proofKind, expires: request.Run.CreatedAt.Add(VolatileLifetime)}
	if request.Kind == assistant.Logs && project.Logs != nil {
		evidence.logConsent = request.LogConsent
		evidence.logPolicy = project.Logs.PolicyFingerprint()
		if err := reader.verifyLogPolicy(work, request.Actor, request.Run, source, evidence); err != nil {
			return Evidence{}, err
		}
	}
	return evidence, nil
}

func (reader *Reader) collect(ctx context.Context, source Source, request ReadRequest, resources []domain.ResourceSnapshot, project *contextbuilder.Request) error {
	scope := request.Scope
	switch request.Kind {
	case assistant.Inventory:
		project.Resources = resources
		for _, resource := range resources {
			if resource.Status.State == domain.AvailabilityStale {
				project.Status = resource.Status
			}
		}
	case assistant.Health:
		query := domain.HealthQuery{Source: scope.Source, Resources: slices.Clone(scope.Resources)}
		result, err := source.Read.GetHealth(ctx, query)
		if err != nil {
			return err
		}
		if result.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		project.Status, project.Health = result.Status, result.Snapshots
	case assistant.Metrics:
		query := domain.MetricHistoryQuery{Source: scope.Source, Resources: slices.Clone(scope.Resources), Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount}, Range: scope.Range, PointLimit: MaximumRecords}
		result, err := source.Read.GetMetricHistory(ctx, query)
		if err != nil {
			return err
		}
		if result.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		project.Status, project.Metrics = result.Status, result.Samples
		project.MoreAvailable = len(result.Samples) == MaximumRecords
	case assistant.Events:
		query := domain.EventQuery{Source: scope.Source, Resources: slices.Clone(scope.Resources), Range: scope.Range, Page: domain.PageRequest{Limit: MaximumRecords}}
		page, err := source.Read.ListEvents(ctx, query)
		if err != nil {
			return err
		}
		if page.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		project.Status, project.Events, project.MoreAvailable = page.Status, page.Events, page.Page.HasMore
	case assistant.Topology:
		query := domain.TopologyQuery{Source: scope.Source, Resources: slices.Clone(scope.Resources), Page: domain.PageRequest{Limit: MaximumRecords}}
		page, err := source.Read.GetRelationships(ctx, query)
		if err != nil {
			return err
		}
		if page.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		project.Status, project.MoreAvailable = page.Status, page.Page.HasMore
		allowed := make(map[domain.ResourceID]bool, len(scope.Resources))
		for _, ref := range scope.Resources {
			allowed[ref.ID] = true
		}
		for _, edge := range page.Relationships {
			if allowed[edge.From] && allowed[edge.To] {
				project.Relationships = append(project.Relationships, edge)
			} else {
				project.MoreAvailable = true
			}
		}
	case assistant.Logs:
		query := logQuery(scope)
		capture, err := source.Logs.Capture(ctx, query)
		if err != nil {
			return err
		}
		_, page, valid := capture.Snapshot()
		if !valid {
			return ErrUnavailable
		}
		project.Status, project.Logs, project.LogConsent, project.MoreAvailable = page.Status, &capture, true, page.Page.HasMore
	default:
		return ErrInvalid
	}
	return nil
}

func logQuery(scope contextbuilder.Scope) domain.LogReadQuery {
	query := domain.LogReadQuery{Source: scope.Source, Range: scope.Range, LineLimit: min(contextbuilder.MaximumLogLines, MaximumRecords), ByteLimit: contextbuilder.MaximumLogBytes}
	for _, ref := range scope.Resources {
		query.Targets = append(query.Targets, domain.LogTarget{Resource: ref})
	}
	return query
}

func (reader *Reader) verifyLogPolicy(ctx context.Context, actor assistant.Actor, run assistant.Run, source Source, evidence Evidence) error {
	if evidence.kind != assistant.Logs {
		return nil
	}
	if !evidence.logConsent || evidence.logPolicy == [32]byte{} || source.Logs == nil {
		return ErrDenied
	}
	capture, err := source.Logs.Capture(ctx, logQuery(evidence.scope))
	if err != nil {
		return safeError(ctx, err)
	}
	_, page, valid := capture.Snapshot()
	if !valid || page.Status.State != domain.AvailabilityAvailable || page.Status.ObservedAt.After(reader.time()) || evidence.scope.Source.Provider != domain.ProviderDemo && reader.time().Sub(page.Status.ObservedAt) > MaximumAuthorityAge || capture.PolicyFingerprint() != evidence.logPolicy {
		return ErrDenied
	}
	if err := reader.gate.Check(ctx, actor, run, reader.time()); err != nil {
		return safeError(ctx, err)
	}
	if ctx.Err() != nil {
		return ErrCancelled
	}
	return nil
}
