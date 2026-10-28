package assistantservice

import (
	"context"
	"errors"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

func (service *Service) begin(ctx context.Context, entry *memoryRun, kind assistant.StepKind, maximum int64) (assistant.Step, error) {
	unlock, err := service.lockRun(ctx, entry)
	if err != nil {
		return assistant.Step{}, err
	}
	defer unlock()
	work, cancel := service.ioContext(ctx)
	defer cancel()
	if err := service.authorizeLocked(work, entry); err != nil {
		return assistant.Step{}, err
	}
	service.mu.Lock()
	run := entry.run
	service.mu.Unlock()
	if kind != assistant.Inference && run.ToolCalls >= assistant.MaximumToolCalls || kind == assistant.Inference && run.InferenceCalls >= assistant.MaximumInferenceCalls {
		return assistant.Step{}, assistant.ErrQuota
	}
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
	unlock, err := service.lockRun(ctx, entry)
	if err != nil {
		return err
	}
	defer unlock()
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

func (service *Service) collect(ctx context.Context, entry *memoryRun) ([]inference.Evidence, error) {
	service.mu.Lock()
	input, run := entry.input, entry.run
	input.Scope = cloneScope(input.Scope)
	revoked := entry.revoked
	service.mu.Unlock()
	if revoked || ctx.Err() != nil {
		return nil, assistant.ErrDenied
	}
	input.Scope.ObservedAt = service.time()
	work, cancel := service.evidenceContext(ctx)
	evidence, err := service.collector.Read(work, assistantevidence.ReadRequest{Actor: input.Admission.Actor, Run: run, Scope: input.Scope, Kind: assistant.Inventory, Destination: contextbuilder.Local, LogConsent: false})
	cancel()
	if err != nil {
		if errors.Is(err, assistantevidence.ErrDenied) {
			return nil, assistant.ErrDenied
		}
		if errors.Is(err, assistantevidence.ErrBudget) {
			return nil, assistant.ErrQuota
		}
		return nil, assistant.ErrUnavailable
	}
	items, err := evidence.Bundle().Items()
	references := evidence.References()
	if err != nil || len(items) != 1 || len(references) != 1 {
		return nil, assistant.ErrDenied
	}
	ref := references[0]
	if ref.Kind != "resource" || ref.ResourceID != input.Scope.Resources[0].ID || ref.Source != input.Scope.Source || ref.ConfigurationVersion != input.Scope.ConfigurationVersion || ref.PolicyVersion != input.Scope.PolicyVersion || ref.RestoreGeneration != input.Scope.RestoreGeneration || ref.Range != input.Scope.Range || ref.ID != items[0].ID {
		return nil, assistant.ErrDenied
	}
	service.mu.Lock()
	if entry.revoked || service.closed {
		service.mu.Unlock()
		return nil, assistant.ErrDenied
	}
	entry.evidence = &evidence
	service.mu.Unlock()
	if err := service.authorizeContent(ctx, entry); err != nil {
		return nil, err
	}
	result := make([]inference.Evidence, len(items))
	for index, item := range items {
		result[index] = inference.Evidence{ID: item.ID, Text: item.Text}
	}
	return result, nil
}

func (service *Service) finishCompleted(ctx context.Context, entry *memoryRun) (assistant.Run, error) {
	unlock, err := service.lockRun(ctx, entry)
	if err != nil {
		return assistant.Run{}, err
	}
	defer unlock()
	service.mu.Lock()
	run := entry.run
	entry.finishing = true
	revoked := entry.revoked
	service.mu.Unlock()
	if revoked || ctx.Err() != nil {
		return assistant.Run{}, assistant.ErrDenied
	}
	work, cancel := service.ioContext(ctx)
	defer cancel()
	completed, err := service.repository.FinishAssistant(work, assistant.Finish{Lease: service.lease(run), State: assistant.Completed})
	if err != nil {
		return assistant.Run{}, ownedError(err)
	}
	service.mu.Lock()
	entry.run = completed
	service.mu.Unlock()
	return completed, nil
}

func (service *Service) execute(ctx context.Context, entry *memoryRun) error {
	read, err := service.begin(ctx, entry, assistant.Inventory, 0)
	if err != nil {
		return err
	}
	evidence, err := service.collect(ctx, entry)
	if err != nil {
		return err
	}
	defer clear(evidence)
	if err = service.completeStep(ctx, entry, read, 0, false); err != nil {
		return err
	}
	step, err := service.begin(ctx, entry, assistant.Inference, service.maximumUnits())
	if err != nil {
		return err
	}
	service.mu.Lock()
	input := entry.input
	service.mu.Unlock()
	// Preparing exact tokens already sends bounded content to the local service.
	// Recheck its sealed read family after reservation and before that first I/O.
	if err = service.authorizeContent(ctx, entry); err != nil {
		return err
	}
	prepared, err := service.gateway.Prepare(ctx, inference.Request{Locale: input.Locale, Question: input.Question, Evidence: evidence, OutputLimit: service.output})
	if err != nil {
		_ = service.completeStep(ctx, entry, step, 0, false)
		return assistant.ErrUnavailable
	}
	if err = service.authorizeContent(ctx, entry); err != nil {
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
	if err = service.authorizeContent(ctx, entry); err != nil {
		return err
	}
	if err = service.completeStep(ctx, entry, step, units, false); err != nil {
		return err
	}
	completed, err := service.finishCompleted(ctx, entry)
	if err != nil {
		return err
	}
	if err = service.authorizeContent(ctx, entry); err != nil {
		return err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if entry.revoked || service.closed || ctx.Err() != nil || !service.time().Before(entry.expires) || entry.evidence == nil {
		return assistant.ErrDenied
	}
	entry.run, entry.answer, entry.citations = completed, cloneAnswer(result.Answer), entry.evidence.References()
	return nil
}
