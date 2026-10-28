package assistantconversation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type runtimeStub struct {
	input    assistantservice.Input
	calls    int
	onSubmit func(assistantservice.Input) (assistant.Run, error)
	Runtime
}

func (stub *runtimeStub) Submit(_ context.Context, input assistantservice.Input) (assistant.Run, error) {
	stub.calls++
	stub.input = input
	if stub.onSubmit != nil {
		return stub.onSubmit(input)
	}
	a := input.Admission
	return assistant.Run{ID: a.ID, ConversationID: a.ConversationID, OwnerID: a.Actor.UserID, SourceID: a.SourceID, ResourceID: a.ResourceID, From: a.From, Until: a.Until}, nil
}

type lookupFunc func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error)

func (lookup lookupFunc) GetAssistant(ctx context.Context, actor assistant.Actor, id string, at time.Time) (assistant.Run, error) {
	return lookup(ctx, actor, id, at)
}

type resolveFunc func(context.Context, assistant.Actor, domain.SourceID, domain.ResourceID, domain.TimeRange) (contextbuilder.Scope, error)

func (resolve resolveFunc) Resolve(ctx context.Context, actor assistant.Actor, source domain.SourceID, resource domain.ResourceID, selected domain.TimeRange) (contextbuilder.Scope, error) {
	return resolve(ctx, actor, source, resource, selected)
}

func conversationFixture(t *testing.T) (Options, Request, assistant.Actor, *runtimeStub) {
	t.Helper()
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"}
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "fixture"}, "native_fixture")
	if err != nil {
		t.Fatal("create selected fixture identity")
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal("create selected fixture reference")
	}
	request := Request{SourceID: source.SourceID, ResourceID: ref.ID, WindowMinutes: 15, Locale: "en", Question: "Explain selected resource."}
	actor := assistant.Actor{UserID: "user_fixture", SessionDigest: [32]byte{1}, DigestKeyID: "fixture_key", AuthRevision: 1}
	runtime := &runtimeStub{}
	options := Options{Runtime: runtime, Repository: lookupFunc(func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error) {
		return assistant.Run{}, assistant.ErrDenied
	}), Resolver: resolveFunc(func(_ context.Context, _ assistant.Actor, s domain.SourceID, r domain.ResourceID, selected domain.TimeRange) (contextbuilder.Scope, error) {
		if s != source.SourceID || r != ref.ID {
			return contextbuilder.Scope{}, assistant.ErrDenied
		}
		return contextbuilder.Scope{Source: source, ConfigurationVersion: 1, PolicyVersion: 1, Range: selected, Resources: []domain.ResourceRef{ref}}, nil
	}), Binding: Binding{ProfileID: "profile", ProfileVersion: 1, PolicyVersion: 1, ReservedUnits: 16128}, Key: append([]byte{1}, make([]byte, 31)...), Now: func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 1000, time.UTC) }}
	return options, request, actor, runtime
}

func TestAdmissionDerivesServerIdentitiesAndNeverAcceptsBrowserScope(t *testing.T) {
	options, request, actor, runtime := conversationFixture(t)
	service, err := New(options)
	if err != nil {
		t.Fatal("construct conversation service")
	}
	defer service.Close()
	nonce := strings.Repeat("c", 64)
	run, err := service.Submit(context.Background(), actor, request, nonce)
	if err != nil {
		t.Fatal("submit bounded resource question")
	}
	a := runtime.input.Admission
	if !strings.HasPrefix(run.ID, "ar_") || len(run.ID) != 67 || !conversationID.MatchString(run.ConversationID) || a.Actor != actor || a.ProfileID != "profile" || a.ProfileVersion != 1 || a.PolicyVersion != 1 || a.Paid || a.IdempotencyDigest == [32]byte{} || a.RequireExisting || runtime.input.RequireExistingConversation || runtime.calls != 1 || !a.From.Equal(options.Now().Add(-15*time.Minute)) || !a.Until.Equal(options.Now()) {
		t.Fatal("server identity, scope, actor or budget mapping changed")
	}
	second, err := service.Submit(context.Background(), actor, request, nonce)
	if err != nil || second.ID != run.ID || second.ConversationID != run.ConversationID {
		t.Fatal("stable owner/nonce mapping changed")
	}
	actor.UserID = "another_user"
	other, err := service.Submit(context.Background(), actor, request, nonce)
	if err != nil || other.ID == run.ID || other.ConversationID == run.ConversationID {
		t.Fatal("different owners shared opaque identities")
	}
}

func TestRetainedRetryFreezesOriginalRangeAndCarriesExistingGuard(t *testing.T) {
	options, request, actor, runtime := conversationFixture(t)
	var saved assistant.Run
	options.Repository = lookupFunc(func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error) {
		if saved.ID == "" {
			return assistant.Run{}, assistant.ErrDenied
		}
		return saved, nil
	})
	service, err := New(options)
	if err != nil {
		t.Fatal("construct retained retry fixture")
	}
	defer service.Close()
	saved, err = service.Submit(context.Background(), actor, request, strings.Repeat("c", 64))
	if err != nil {
		t.Fatal("initial trusted run failed")
	}
	original := runtime.input
	service.now = func() time.Time { return options.Now().Add(time.Minute) }
	if _, err = service.Submit(context.Background(), actor, request, strings.Repeat("c", 64)); err != nil || !runtime.input.Admission.RequireExisting || runtime.input.Admission.From != original.Admission.From || runtime.input.Admission.Until != original.Admission.Until || runtime.input.Admission.ConversationID != original.Admission.ConversationID {
		t.Fatal("retained retry changed pinned scope or lost race guard")
	}
	request.WindowMinutes = 5
	if _, err = service.Submit(context.Background(), actor, request, strings.Repeat("c", 64)); err != assistant.ErrConflict || runtime.calls != 2 {
		t.Fatal("changed selected window reached runtime")
	}
}

func TestExplicitConversationPresenceReachesAtomicRuntimeGuard(t *testing.T) {
	options, request, actor, runtime := conversationFixture(t)
	service, err := New(options)
	if err != nil {
		t.Fatal("construct explicit fixture")
	}
	defer service.Close()
	request.ConversationID = "ac_" + strings.Repeat("d", 64)
	if _, err = service.Submit(context.Background(), actor, request, strings.Repeat("c", 64)); err != nil || !runtime.input.RequireExistingConversation || runtime.input.Admission.ConversationID != request.ConversationID {
		t.Fatal("explicit conversation was replaced or lost its trusted guard")
	}
}

func TestResolverCannotExpandSourceResourceRangeOrPolicy(t *testing.T) {
	for name, mutate := range map[string]func(*contextbuilder.Scope){"source": func(s *contextbuilder.Scope) { s.Source.SourceID = "another_source" }, "resource": func(s *contextbuilder.Scope) { s.Resources = nil }, "range": func(s *contextbuilder.Scope) { s.Range.End = s.Range.End.Add(time.Microsecond) }, "epoch": func(s *contextbuilder.Scope) { s.ConfigurationVersion = 0 }, "policy": func(s *contextbuilder.Scope) { s.PolicyVersion = 2 }, "restore": func(s *contextbuilder.Scope) { s.RestoreGeneration = -1 }} {
		t.Run(name, func(t *testing.T) {
			options, request, actor, runtime := conversationFixture(t)
			base := options.Resolver
			options.Resolver = resolveFunc(func(ctx context.Context, a assistant.Actor, s domain.SourceID, r domain.ResourceID, selected domain.TimeRange) (contextbuilder.Scope, error) {
				scope, err := base.Resolve(ctx, a, s, r, selected)
				mutate(&scope)
				return scope, err
			})
			service, err := New(options)
			if err != nil {
				t.Fatal("construct malformed resolver fixture")
			}
			defer service.Close()
			if _, err = service.Submit(context.Background(), actor, request, strings.Repeat("c", 64)); err != assistant.ErrUnavailable || runtime.calls != 0 {
				t.Fatal("expanded resolver scope reached runtime")
			}
		})
	}
}

func TestInvalidInputsAndClosedCoordinatorCannotDispatch(t *testing.T) {
	options, request, actor, runtime := conversationFixture(t)
	service, err := New(options)
	if err != nil {
		t.Fatal("construct invalid input fixture")
	}
	for _, nonce := range []string{"", strings.Repeat("c", 63), strings.Repeat("C", 64), strings.Repeat("c", 64) + "\n"} {
		if _, err = service.Submit(context.Background(), actor, request, nonce); err != assistant.ErrInvalid {
			t.Fatal("invalid nonce accepted")
		}
	}
	options.Repository = lookupFunc(func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error) {
		return assistant.Run{}, errors.New("private repository diagnostic")
	})
	failing, err := New(options)
	if err != nil {
		t.Fatal("construct repository fault fixture")
	}
	defer failing.Close()
	if _, err = failing.Submit(context.Background(), actor, request, strings.Repeat("c", 64)); err != assistant.ErrUnavailable {
		t.Fatal("repository fault escaped owned error")
	}
	service.Close()
	service.Close()
	if _, err = service.Submit(context.Background(), actor, request, strings.Repeat("c", 64)); err != assistant.ErrUnavailable || runtime.calls != 0 || service.key != [32]byte{} {
		t.Fatal("closed coordinator admitted input or retained key")
	}
}

func TestConstructionRejectsMissingPortsAndUnsafeBindingWithoutIO(t *testing.T) {
	for name, mutate := range map[string]func(*Options){"runtime": func(o *Options) { o.Runtime = nil }, "repository": func(o *Options) { o.Repository = nil }, "resolver": func(o *Options) { o.Resolver = nil }, "clock": func(o *Options) { o.Now = nil }, "key_length": func(o *Options) { o.Key = make([]byte, 31) }, "zero_key": func(o *Options) { o.Key = make([]byte, 32) }, "profile": func(o *Options) { o.Binding.ProfileID = "" }, "profile_epoch": func(o *Options) { o.Binding.ProfileVersion = 0 }, "policy_epoch": func(o *Options) { o.Binding.PolicyVersion = 0 }, "budget": func(o *Options) { o.Binding.ReservedUnits = 1 }, "budget_overflow": func(o *Options) { o.Binding.ReservedUnits = assistant.MaximumReservationUnits + 1 }} {
		t.Run(name, func(t *testing.T) {
			options, _, _, runtime := conversationFixture(t)
			mutate(&options)
			service, err := New(options)
			if service != nil || err != assistant.ErrInvalid || runtime.calls != 0 {
				t.Fatal("invalid construction created a capability or performed work")
			}
		})
	}
}

func TestCancelledResolutionCannotEnterRuntime(t *testing.T) {
	options, request, actor, runtime := conversationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := options.Resolver
	options.Resolver = resolveFunc(func(ctx context.Context, a assistant.Actor, s domain.SourceID, r domain.ResourceID, selected domain.TimeRange) (contextbuilder.Scope, error) {
		scope, err := base.Resolve(ctx, a, s, r, selected)
		cancel()
		return scope, err
	})
	service, err := New(options)
	if err != nil {
		t.Fatal("construct cancelled resolution fixture")
	}
	defer service.Close()
	if _, err = service.Submit(ctx, actor, request, strings.Repeat("c", 64)); err != assistant.ErrUnavailable || runtime.calls != 0 {
		t.Fatal("cancelled resolution admitted a late question")
	}
}
