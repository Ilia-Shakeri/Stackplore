package assistantevidence

import (
	"context"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (reader *Reader) validate(actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) (Source, error) {
	if reader == nil || !actor.Valid() || !assistant.Identifier(run.ID) || run.OwnerID != actor.UserID || !run.Goal.Valid() || run.Paid ||
		scope.RunID.Validate() != nil || string(scope.RunID) != run.ID || scope.Source.Validate() != nil || string(scope.Source.SourceID) != run.SourceID ||
		scope.ConfigurationVersion < 1 || scope.ConfigurationVersion != run.SourceVersion || scope.PolicyVersion < 1 || scope.PolicyVersion != run.PolicyVersion ||
		scope.RestoreGeneration < 0 || scope.RestoreGeneration != run.RestoreGeneration || !assistant.Identifier(run.ProfileID) || run.ProfileVersion < 1 ||
		scope.Range.ValidateLookback(24*time.Hour) != nil || scope.Range.Start != run.From || scope.Range.End != run.Until || run.Version < 0 ||
		!assistant.Timestamp(scope.ObservedAt) || scope.ObservedAt.Before(scope.Range.End) || scope.ObservedAt.After(reader.time()) || len(scope.Resources) < 1 || len(scope.Resources) > MaximumResources {
		return Source{}, ErrInvalid
	}
	source, found := reader.sources[scope.Source.SourceID]
	if !found || source.Reference != scope.Source || source.ConfigurationVersion != scope.ConfigurationVersion {
		return Source{}, ErrDenied
	}
	seen := make(map[domain.ResourceID]bool, len(scope.Resources))
	for _, ref := range scope.Resources {
		if ref.Validate() != nil || !scope.Source.Matches(ref) || seen[ref.ID] {
			return Source{}, ErrInvalid
		}
		seen[ref.ID] = true
	}
	if run.ResourceID != "" && (len(scope.Resources) != 1 || string(scope.Resources[0].ID) != run.ResourceID) {
		return Source{}, ErrDenied
	}
	if run.Goal == assistant.ExplainResource && run.ResourceID == "" {
		return Source{}, ErrInvalid
	}
	return source, nil
}

// AuthorizeAdmission checks only metadata for a trusted candidate. It never
// grants access to an existing run or performs a provider read.
func (reader *Reader) AuthorizeAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	work, cancel, err := reader.metadataWork(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	scope.Resources = slices.Clone(scope.Resources)
	_, err = reader.validate(actor, run, scope)
	if err != nil {
		return err
	}
	if run.Version != 0 || run.ConfigurationVersion != 0 || run.Fence != 0 || run.State != assistant.Queued {
		return ErrDenied
	}
	if err := reader.gate.CheckAdmission(work, actor, run, reader.time()); err != nil {
		return safeError(work, err)
	}
	if work.Err() != nil {
		return ErrCancelled
	}
	return nil
}

// Authorize checks metadata for a persisted run. It never releases content or
// reads a provider. Previously collected content requires AuthorizeEvidence.
func (reader *Reader) Authorize(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	work, cancel, err := reader.metadataWork(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	scope = cloneScope(scope)
	_, err = reader.validate(actor, run, scope)
	if err != nil {
		return err
	}
	if run.Version < 1 || run.ConfigurationVersion < 1 {
		return ErrDenied
	}
	if err := reader.gate.Check(work, actor, run, reader.time()); err != nil {
		return safeError(work, err)
	}
	if work.Err() != nil {
		return ErrCancelled
	}
	return nil
}

func (reader *Reader) fence(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope, source Source, kind assistant.StepKind, logConsent bool) ([]domain.ResourceSnapshot, error) {
	if err := reader.gate.Check(ctx, actor, run, reader.time()); err != nil {
		return nil, safeError(ctx, err)
	}
	if err := source.Live.Check(ctx, cloneScope(scope), kind, logConsent); err != nil {
		return nil, safeError(ctx, err)
	}
	resources := make([]domain.ResourceSnapshot, 0, len(scope.Resources))
	for _, ref := range scope.Resources {
		if ctx.Err() != nil {
			return nil, ErrCancelled
		}
		query := domain.ResourceDetailQuery{Source: scope.Source, ResourceID: ref.ID}
		resource, err := source.Read.GetResource(ctx, query)
		if err != nil {
			return nil, safeError(ctx, err)
		}
		if resource.ValidateFor(query) != nil || resource.Resource.Ref != ref || !resource.Status.AllowsData() {
			return nil, ErrDenied
		}
		resources = append(resources, resource)
	}
	if err := reader.gate.Check(ctx, actor, run, reader.time()); err != nil {
		return nil, safeError(ctx, err)
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled
	}
	return resources, nil
}

func cloneScope(scope contextbuilder.Scope) contextbuilder.Scope {
	scope.Resources = slices.Clone(scope.Resources)
	return scope
}
