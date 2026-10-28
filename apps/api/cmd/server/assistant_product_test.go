package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type assistantProductFixture struct {
	assistantProductRepository
	mu                 sync.Mutex
	settings           assistant.Settings
	source             domain.SourceConfiguration
	updates            int
	configurationError bool
	block              bool
	started            chan struct{}
	cancelled          chan struct{}
	release            chan struct{}
	active             atomic.Int32
}

func (fixture *assistantProductFixture) GetAssistantSettings(context.Context) (assistant.Settings, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.settings, nil
}
func (fixture *assistantProductFixture) ConfigureAssistant(_ context.Context, settings assistant.Settings, audit assistant.OperatorAudit) (assistant.Settings, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.configurationError || !audit.Valid() || settings.Version != fixture.settings.Version {
		return assistant.Settings{}, assistant.ErrConflict
	}
	settings.Version++
	fixture.settings = settings
	fixture.updates++
	return settings, nil
}
func (fixture *assistantProductFixture) GetSourceConfiguration(_ context.Context, id domain.SourceID) (domain.SourceConfiguration, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if id != fixture.source.Source.ID {
		return domain.SourceConfiguration{}, domain.ErrRepositoryNotFound
	}
	return fixture.source, nil
}
func (*assistantProductFixture) AssistantRestoreGeneration(context.Context) (int64, error) {
	return 0, nil
}
func (fixture *assistantProductFixture) RecoverAssistant(ctx context.Context, _ time.Time, _ int) (int, error) {
	if !fixture.block {
		return 0, ctx.Err()
	}
	fixture.active.Add(1)
	defer fixture.active.Add(-1)
	select {
	case fixture.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	select {
	case fixture.cancelled <- struct{}{}:
	default:
	}
	<-fixture.release
	return 0, ctx.Err()
}
func (*assistantProductFixture) ClaimAssistant(ctx context.Context, _ assistant.Claim) (assistant.Run, bool, error) {
	return assistant.Run{}, false, ctx.Err()
}

type assistantProductDatabaseFixture struct {
	databaseRuntime
	*assistantProductFixture
}

func (*assistantProductDatabaseFixture) AuthDigestKey() (string, []byte, error) {
	return "key_fixture", bytes.Repeat([]byte{7}, 32), nil
}

func assistantEnabledSettings(t *testing.T, origin string) config.Config {
	t.Helper()
	settings, err := config.Load(mapGetter{"STACKPLORE_ASSISTANT_ENABLED": "true", "STACKPLORE_ASSISTANT_ORIGIN": origin, "STACKPLORE_ASSISTANT_MODEL_SHA256": strings.Repeat("a", 64), "STACKPLORE_ASSISTANT_BUILD": "synthetic-build-1", "STACKPLORE_ASSISTANT_TEMPLATE_SHA256": strings.Repeat("b", 64), "STACKPLORE_ASSISTANT_CONTEXT_TOKENS": "24576", "STACKPLORE_ASSISTANT_GENERATION": "1", "STACKPLORE_ASSISTANT_OUTPUT_TOKENS": "128"})
	if err != nil {
		t.Fatal("construct enabled fixture profile")
	}
	return settings
}

func assistantProductFixturePorts(t *testing.T) (*assistantProductDatabaseFixture, []httpapi.ProductProvider) {
	t.Helper()
	provider, err := demo.New()
	if err != nil {
		t.Fatal("construct approved demo boundary")
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := &assistantProductFixture{settings: assistant.Settings{Version: 1, PolicyVersion: 1, ProfileID: "disabled", ProfileVersion: 1, UserQuota: 1000000, InstallationQuota: 10000000}, source: domain.SourceConfiguration{Source: provider.Source(), ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}, started: make(chan struct{}, 2), cancelled: make(chan struct{}, 2), release: make(chan struct{})}
	return &assistantProductDatabaseFixture{databaseRuntime: &fakeDatabase{}, assistantProductFixture: fixture}, []httpapi.ProductProvider{provider}
}

func TestAssistantProductDefaultOffDoesNotTouchAnyPorts(t *testing.T) {
	if runtime, err := buildAssistantProduct(nil, defaultSettings(t), nil, nil, nil, nil); runtime != nil || err != nil {
		t.Fatal("disabled composition touched runtime ports")
	}
	(*assistantProductRuntime)(nil).Close()
}

func TestAssistantProductReconcilesOnceWithoutProcessorRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	settings := assistantEnabledSettings(t, server.URL)
	database, providers := assistantProductFixturePorts(t)
	for range 2 {
		runtime, err := buildAssistantProduct(context.Background(), settings, database, providers, providers, discardLogger(t))
		if err != nil || runtime == nil {
			t.Fatal("enabled composition failed before idle worker")
		}
		runtime.Close()
		runtime.Close()
		select {
		case <-runtime.done:
		default:
			t.Fatal("close did not join producers")
		}
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	if database.updates != 1 || database.settings.Version != 2 || !database.settings.Enabled || database.settings.PaidEnabled || database.settings.ProfilePaid || database.settings.UserQuota != 1000000 || database.settings.InstallationQuota != 10000000 || requests.Load() != 0 {
		t.Fatal("restart mutated usage policy or contacted processor without admission")
	}
}

func TestAssistantProductCloseWaitsForCancelledRepositoryWork(t *testing.T) {
	database, providers := assistantProductFixturePorts(t)
	database.block = true
	var release sync.Once
	defer release.Do(func() { close(database.release) })
	runtime, err := buildAssistantProduct(context.Background(), assistantEnabledSettings(t, "http://127.0.0.1:59641"), database, providers, providers, discardLogger(t))
	if err != nil {
		t.Fatal("construct joined runtime fixture")
	}
	select {
	case <-database.started:
	case <-time.After(time.Second):
		t.Fatal("owned producer did not start")
	}
	done := make(chan struct{})
	go func() { runtime.Close(); close(done) }()
	select {
	case <-database.cancelled:
	case <-time.After(time.Second):
		t.Fatal("close did not cancel repository work")
	}
	select {
	case <-done:
		t.Fatal("close returned before producer released")
	default:
	}
	release.Do(func() { close(database.release) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled producer was not joined")
	}
	if database.active.Load() != 0 {
		t.Fatal("repository work survived runtime close")
	}
}

func TestAssistantProductRefusesUnsafeCompositionAndControlDowngrades(t *testing.T) {
	settings := assistantEnabledSettings(t, "http://127.0.0.1:59641")
	database, providers := assistantProductFixturePorts(t)
	for _, mutate := range []func(*config.Config, *assistantProductDatabaseFixture){
		func(c *config.Config, _ *assistantProductDatabaseFixture) { c.GuestDemo = true },
		func(_ *config.Config, d *assistantProductDatabaseFixture) { d.settings.PolicyVersion = 2 },
		func(_ *config.Config, d *assistantProductDatabaseFixture) { d.settings.ProfileVersion = 2 },
		func(_ *config.Config, d *assistantProductDatabaseFixture) { d.configurationError = true },
		func(_ *config.Config, d *assistantProductDatabaseFixture) {
			d.source.ProfileID = domain.SourceProfileDockerRestrictedProxy
		},
	} {
		current := settings
		database, providers = assistantProductFixturePorts(t)
		mutate(&current, database)
		if runtime, err := buildAssistantProduct(context.Background(), current, database, providers, providers, discardLogger(t)); runtime != nil || !errors.Is(err, errAssistantStart) {
			if runtime != nil {
				runtime.Close()
			}
			t.Fatal("unsafe runtime composition accepted")
		}
	}
	if runtime, err := buildAssistantProduct(context.Background(), settings, &fakeDatabase{}, providers, providers, discardLogger(t)); runtime != nil || err != errAssistantStart {
		t.Fatal("missing complete product repository accepted")
	}
}

func TestAssistantProductBindingChangesWithPinsAndLimits(t *testing.T) {
	first := assistantEnabledSettings(t, "http://127.0.0.1:59641")
	second := assistantEnabledSettings(t, "http://127.0.0.1:59642")
	a, err := assistantProductBinding(first.Assistant)
	if err != nil {
		t.Fatal("bind first profile")
	}
	b, err := assistantProductBinding(second.Assistant)
	if err != nil || a.ProfileID == b.ProfileID || a.ProfileVersion != 1 || a.ReservedUnits != 16128 {
		t.Fatal("profile identity did not bind actual pins and output budget")
	}
	if _, err := assistantProductBinding(config.AssistantProfile{}); err != errAssistantStart {
		t.Fatal("disabled profile produced enabled binding")
	}
}

func TestAssistantProductRetainedOperatorStopSurvivesRestart(t *testing.T) {
	database, providers := assistantProductFixturePorts(t)
	settings := assistantEnabledSettings(t, "http://127.0.0.1:59641")
	runtime, err := buildAssistantProduct(context.Background(), settings, database, providers, providers, discardLogger(t))
	if err != nil || runtime == nil {
		t.Fatal("construct initial enabled fixture")
	}
	runtime.Close()
	database.mu.Lock()
	database.settings.Enabled = false
	database.mu.Unlock()
	if resumed, err := buildAssistantProduct(context.Background(), settings, database, providers, providers, discardLogger(t)); resumed != nil || err != nil {
		if resumed != nil {
			resumed.Close()
		}
		t.Fatal("retained stop did not keep the feature sealed")
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	if database.settings.Enabled || database.updates != 1 {
		t.Fatal("restart overrode an operator stop")
	}
}

func TestAssistantResourceResolverUsesCurrentEpochAndApprovedOpaqueReference(t *testing.T) {
	database, providers := assistantProductFixturePorts(t)
	provider := providers[0].(*demo.Provider)
	ctx := context.Background()
	capture, err := provider.CaptureInventory(ctx)
	if err != nil || len(capture.Resources) == 0 {
		t.Fatal("read controlled fixture inventory")
	}
	ref := capture.Resources[0].Resource.Ref
	source := domain.ProviderRef{Provider: ref.Identity.Provider, SourceID: ref.Identity.SourceID}
	reader, err := assistantevidence.New(assistantevidence.Options{Gate: assistantevidence.RepositoryGate{Repository: database}, Sources: []assistantevidence.Source{{Reference: source, ConfigurationVersion: 1, Read: provider, Live: assistantevidence.CaptureAuthority{Source: source, Provider: provider, Now: time.Now}}}, Now: time.Now})
	if err != nil {
		t.Fatal("construct stable authority reader")
	}
	resolver := &assistantResourceResolver{repository: database, reader: reader, sources: map[domain.SourceID]assistantSelectedSource{source.SourceID: {configuration: database.source, read: provider}}}
	actor := assistant.Actor{UserID: "owner_fixture", SessionDigest: [32]byte{1}, DigestKeyID: "key_fixture", AuthRevision: 1}
	at := time.Now().UTC().Truncate(time.Microsecond)
	selected := domain.TimeRange{Start: at.Add(-15 * time.Minute), End: at}
	for version := int64(1); version <= 2; version++ {
		database.mu.Lock()
		database.source.Version = version
		database.mu.Unlock()
		scope, err := resolver.Resolve(ctx, actor, source.SourceID, ref.ID, selected)
		if err != nil || scope.ConfigurationVersion != version || scope.PolicyVersion != 1 || scope.RestoreGeneration != 0 || scope.Range != selected || len(scope.Resources) != 1 || scope.Resources[0] != ref {
			t.Fatal("trusted selection lost current epoch or exact opaque resource")
		}
	}
	if err := reader.AdvanceSourceVersion(source, 1); err != assistantevidence.ErrDenied {
		t.Fatal("selection did not fence obsolete source metadata")
	}
	database.mu.Lock()
	database.source.Enabled = false
	database.mu.Unlock()
	if _, err := resolver.Resolve(ctx, actor, source.SourceID, ref.ID, selected); err != assistant.ErrDenied {
		t.Fatal("disabled source remained selectable")
	}
	if _, err := (*assistantResourceResolver)(nil).Resolve(ctx, actor, source.SourceID, ref.ID, selected); err != assistant.ErrInvalid {
		t.Fatal("nil selection boundary accepted input")
	}
}
