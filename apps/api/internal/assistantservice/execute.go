package assistantservice

import (
	"context"
	"slices"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

func (service *Service) authorize(ctx context.Context, entry *memoryRun) error {
	service.mu.Lock()
	run, scope, actor, revoked := entry.run, cloneScope(entry.input.Scope), entry.input.Admission.Actor, entry.revoked
	service.mu.Unlock()
	if revoked || ctx.Err() != nil {
		return assistant.ErrDenied
	}
	work, cancel := service.ioContext(ctx)
	defer cancel()
	if service.authorizer.Authorize(work, actor, run, scope) != nil {
		service.invalidateEntry(entry)
		return assistant.ErrDenied
	}
	return nil
}

func (service *Service) begin(ctx context.Context, entry *memoryRun, kind assistant.StepKind, maximum int64) (assistant.Step, error) {
	if err := service.authorize(ctx, entry); err != nil {
		return assistant.Step{}, err
	}
	service.mu.Lock()
	run := entry.run
	service.mu.Unlock()
	if kind != assistant.Inference && run.ToolCalls >= assistant.MaximumToolCalls || kind == assistant.Inference && run.InferenceCalls >= assistant.MaximumInferenceCalls {
		return assistant.Step{}, assistant.ErrQuota
	}
	work, cancel := service.ioContext(ctx)
	defer cancel()
	updated, step, err := service.repository.BeginAssistantStep(work, assistant.StepStart{Lease: service.lease(run), Kind: kind, MaximumUnits: maximum})
	if err != nil {
		return assistant.Step{}, ownedError(err)
	}
	service.mu.Lock()
	entry.run = updated
	revoked := entry.revoked
	service.mu.Unlock()
	if revoked {
		return assistant.Step{}, assistant.ErrDenied
	}
	return step, nil
}

func (service *Service) completeStep(ctx context.Context, entry *memoryRun, step assistant.Step, units int64, uncertain bool) error {
	service.mu.Lock()
	run := entry.run
	service.mu.Unlock()
	work, cancel := service.ioContext(ctx)
	defer cancel()
	updated, err := service.repository.FinishAssistantStep(work, assistant.StepFinish{Lease: service.lease(run), Number: step.Number, Units: units, Uncertain: uncertain})
	if err != nil {
		return ownedError(err)
	}
	service.mu.Lock()
	entry.run = updated
	service.mu.Unlock()
	return nil
}

func (service *Service) execute(ctx context.Context, entry *memoryRun) error {
	read, err := service.begin(ctx, entry, assistant.Inventory, 0)
	if err != nil {
		return err
	}
	service.mu.Lock()
	input := entry.input
	input.Scope = cloneScope(input.Scope)
	service.mu.Unlock()
	input.Scope.ObservedAt = service.time()
	readContext, cancel := context.WithTimeout(ctx, inference.MaximumCallDuration)
	request, err := service.collector.Collect(readContext, cloneScope(input.Scope), assistant.Inventory)
	cancel()
	if err != nil {
		return assistant.ErrUnavailable
	}
	if !sameScope(request.Scope, input.Scope) || request.Destination != contextbuilder.Local || len(request.Resources) != 1 || len(request.Health)+len(request.Metrics)+len(request.Events)+len(request.Relationships) != 0 || request.Logs != nil || request.LogConsent {
		return assistant.ErrDenied
	}
	request.Scope.ObservedAt = service.time()
	request.RecordLimit, request.ByteLimit = 1, 8192
	bundle, err := contextbuilder.Build(ctx, request)
	if err != nil {
		return assistant.ErrDenied
	}
	references := bundle.Citations()
	if len(references) != 1 {
		return assistant.ErrUnavailable
	}
	encoded, err := bundle.JSON()
	if err != nil || len(encoded) > 8192 {
		clear(encoded)
		return assistant.ErrQuota
	}
	defer clear(encoded)
	if err = service.completeStep(ctx, entry, read, 0, false); err != nil {
		return err
	}
	step, err := service.begin(ctx, entry, assistant.Inference, service.maximumUnits())
	if err != nil {
		return err
	}
	prepared, err := service.gateway.Prepare(ctx, inference.Request{Locale: input.Locale, Question: input.Question, Evidence: []inference.Evidence{{ID: references[0].ID, Text: string(encoded)}}, OutputLimit: service.output})
	if err != nil {
		_ = service.completeStep(ctx, entry, step, 0, false)
		return assistant.ErrUnavailable
	}
	if err = service.authorize(ctx, entry); err != nil {
		return err
	}
	result, err := service.gateway.Dispatch(ctx, prepared)
	if err != nil {
		_ = service.completeStep(ctx, entry, step, 0, true)
		return assistant.ErrUnavailable
	}
	units := int64(result.InputTokens + result.OutputTokens)
	if units > step.MaximumUnits || answerBytes(result.Answer) > inference.MaximumOutputBytes {
		_ = service.completeStep(ctx, entry, step, 0, true)
		return assistant.ErrQuota
	}
	if err = service.authorize(ctx, entry); err != nil {
		return err
	}
	if err = service.completeStep(ctx, entry, step, units, false); err != nil {
		return err
	}
	service.mu.Lock()
	run := entry.run
	entry.finishing = true
	service.mu.Unlock()
	work, finish := service.ioContext(ctx)
	completed, err := service.repository.FinishAssistant(work, assistant.Finish{Lease: service.lease(run), State: assistant.Completed})
	finish()
	if err != nil {
		return ownedError(err)
	}
	service.mu.Lock()
	entry.run = completed
	service.mu.Unlock()
	if err = service.authorize(ctx, entry); err != nil {
		return err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if entry.revoked || service.closed || ctx.Err() != nil || !service.time().Before(entry.expires) {
		return assistant.ErrDenied
	}
	entry.run, entry.answer, entry.citations = completed, cloneAnswer(result.Answer), references
	return nil
}

func sameScope(first, second contextbuilder.Scope) bool {
	return first.RunID == second.RunID && first.Source == second.Source && first.ConfigurationVersion == second.ConfigurationVersion && first.PolicyVersion == second.PolicyVersion && first.RestoreGeneration == second.RestoreGeneration && first.Range == second.Range && first.ObservedAt == second.ObservedAt && slices.Equal(first.Resources, second.Resources)
}
