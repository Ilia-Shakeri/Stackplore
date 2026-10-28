package assistantservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
)

type faultRepository struct {
	*fakeRepository
	beginKind   assistant.StepKind
	finishKind  assistant.StepKind
	finishRun   bool
	afterFinish func()
}

func (repository *faultRepository) BeginAssistantStep(ctx context.Context, input assistant.StepStart) (assistant.Run, assistant.Step, error) {
	if input.Kind == repository.beginKind {
		return assistant.Run{}, assistant.Step{}, assistant.ErrQuota
	}
	return repository.fakeRepository.BeginAssistantStep(ctx, input)
}

func (repository *faultRepository) FinishAssistantStep(ctx context.Context, input assistant.StepFinish) (assistant.Run, error) {
	repository.mu.Lock()
	kind := repository.pending[input.Lease.RunID].Kind
	repository.mu.Unlock()
	if kind == repository.finishKind {
		return assistant.Run{}, errors.New("private storage detail")
	}
	return repository.fakeRepository.FinishAssistantStep(ctx, input)
}

func (repository *faultRepository) FinishAssistant(ctx context.Context, input assistant.Finish) (assistant.Run, error) {
	if repository.finishRun && input.State == assistant.Completed {
		return assistant.Run{}, assistant.ErrConflict
	}
	run, err := repository.fakeRepository.FinishAssistant(ctx, input)
	if input.State == assistant.Completed && repository.afterFinish != nil {
		repository.afterFinish()
	}
	return run, err
}

type authorizerFuncHook func(context.Context, assistant.Actor, assistant.Run, contextbuilder.Scope) error

type authorizerFunc struct {
	Authorizer
	hook authorizerFuncHook
}

func withAuthorizer(base Authorizer, hook authorizerFuncHook) Authorizer {
	return authorizerFunc{Authorizer: base, hook: hook}
}

func (authorize authorizerFunc) AuthorizeAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	if err := authorize.Authorizer.AuthorizeAdmission(ctx, actor, run, scope); err != nil {
		return err
	}
	return authorize.hook(ctx, actor, run, scope)
}

func (authorize authorizerFunc) Authorize(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	if err := authorize.Authorizer.Authorize(ctx, actor, run, scope); err != nil {
		return err
	}
	return authorize.hook(ctx, actor, run, scope)
}

func (authorize authorizerFunc) AuthorizeEvidence(ctx context.Context, actor assistant.Actor, run assistant.Run, evidence assistantevidence.Evidence) error {
	if err := authorize.Authorizer.AuthorizeEvidence(ctx, actor, run, evidence); err != nil {
		return err
	}
	return authorize.hook(ctx, actor, run, contextbuilder.Scope{})
}

func TestDependencyFailuresNeverPublishOrRetry(t *testing.T) {
	for _, failure := range []string{"read", "projection", "count", "read_finish", "reserve", "usage_finish", "completion", "before_dispatch", "after_completion"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			service, repository, authorizer, model := fixtureService(t, nil)
			fault := &faultRepository{fakeRepository: repository}
			service.repository = fault
			switch failure {
			case "read":
				service.collector = collectorFunc(func(context.Context, assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
					return assistantevidence.Evidence{}, errors.New("private collector detail")
				})
			case "projection":
				authorizer.reads.invalidSynthetic.Store(true)
			case "count":
				model.countError = true
			case "read_finish":
				fault.finishKind = assistant.Inventory
			case "reserve":
				fault.beginKind = assistant.Inference
			case "usage_finish":
				fault.finishKind = assistant.Inference
			case "completion":
				fault.finishRun = true
			case "before_dispatch":
				service.authorizer = withAuthorizer(service.authorizer, func(_ context.Context, _ assistant.Actor, run assistant.Run, _ contextbuilder.Scope) error {
					if run.InferenceCalls > 0 {
						return assistant.ErrDenied
					}
					return nil
				})
			case "after_completion":
				fault.afterFinish = func() { authorizer.denied.Store(true) }
			}
			input := inputFixture(t, 1)
			if _, err := service.Submit(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			if _, err := service.runOne(context.Background(), "worker"); err == nil || strings.Contains(err.Error(), "private") {
				t.Fatal("dependency failure succeeded or leaked private error text")
			}
			service.mu.Lock()
			entry := service.entries[input.Admission.ID]
			cleared := entry.revoked && entry.answer == nil && entry.input.Question == ""
			service.mu.Unlock()
			if !cleared || model.calls.Load() > 1 {
				t.Fatal("dependency failure retained a result or repeated a call")
			}
			if failure == "reserve" || failure == "before_dispatch" {
				if model.calls.Load() != 0 {
					t.Fatal("denied dispatch reached generation")
				}
			}
		})
	}
}

func TestInputScopeAndWorkerConfigurationFailClosed(t *testing.T) {
	t.Parallel()
	var valid Options
	fixtureService(t, func(options *Options, _ *modelFixture) { valid = *options })
	for _, alter := range []func(*Options){
		func(options *Options) { options.Workers = 0 },
		func(options *Options) { options.Workers = 3 },
		func(options *Options) { options.PollInterval = time.Nanosecond },
		func(options *Options) { options.Heartbeat = 11 * time.Second },
		func(options *Options) { options.FingerprintKey = nil },
		func(options *Options) { options.FingerprintKey = make([]byte, 32) },
		func(options *Options) { options.Collector = nil },
		func(options *Options) { options.OutputLimit = 0 },
	} {
		options := valid
		alter(&options)
		if service, err := New(options); service != nil || err != assistant.ErrInvalid {
			t.Fatal("unbounded or incomplete worker configuration accepted")
		}
	}
	service, repository, _, _ := fixtureService(t, nil)
	for _, alter := range []func(*Input){
		func(input *Input) { input.Admission.Paid = true },
		func(input *Input) { input.Admission.Goal = assistant.InvestigateIncident },
		func(input *Input) { input.Question = strings.Repeat("x", 8193) },
		func(input *Input) { input.Scope.Source.SourceID = "other_source" },
		func(input *Input) { input.Scope.PolicyVersion++ },
		func(input *Input) { input.Scope.RestoreGeneration++ },
		func(input *Input) { input.Scope.Resources = nil },
	} {
		input := inputFixture(t, 1)
		alter(&input)
		if _, err := service.Submit(context.Background(), input); err != assistant.ErrInvalid {
			t.Fatal("unsupported material reached metadata admission")
		}
	}
	if repository.admissions != 0 {
		t.Fatal("invalid input reached storage")
	}
	for _, value := range []any{Input{Question: "private question"}, Snapshot{}, valid} {
		if strings.Contains(fmt.Sprintf("%+v", value), "private question") {
			t.Fatal("formatting exposed raw text")
		}
	}
	for _, value := range []any{Input{Question: "private question"}, Snapshot{}} {
		body, err := json.Marshal(value)
		if err != nil || string(body) != "{}" {
			t.Fatal("private value became a durable transport object")
		}
	}
	var absent *Service
	absent.Close()
	absent.InvalidateUser("user")
	if _, err := absent.Get(context.Background(), assistant.Actor{}, "run"); err != assistant.ErrInvalid {
		t.Fatal("nil registry exposed data")
	}
	if _, err := service.Cancel(context.Background(), assistant.Actor{}, "run"); err != assistant.ErrInvalid {
		t.Fatal("invalid actor reached cancellation")
	}
	if _, err := service.Get(context.Background(), inputFixture(t, 1).Admission.Actor, "missing"); err != assistant.ErrDenied {
		t.Fatal("missing run disclosed metadata")
	}
	if err := absent.Run(context.Background()); err != assistant.ErrInvalid {
		t.Fatal("nil service started workers")
	}
}
