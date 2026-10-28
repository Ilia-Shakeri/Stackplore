package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantconversation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	localprocessor "github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/inference"
)

var errAssistantStart = errors.New("helper runtime could not start")

type assistantProductRepository interface {
	assistant.RuntimeRepository
	assistant.AdmissionRepository
	assistantevidence.AuthorityRepository
	GetAssistantSettings(context.Context) (assistant.Settings, error)
	AssistantRestoreGeneration(context.Context) (int64, error)
	GetSourceConfiguration(context.Context, domain.SourceID) (domain.SourceConfiguration, error)
}

type assistantProductRuntime struct {
	service *assistantconversation.Service
	core    *assistantservice.Service
	stop    context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

// Close fences input, cancels content work and joins every producer before the
// application may close storage. It also handles startup failures after creation.
func (runtime *assistantProductRuntime) Close() {
	if runtime == nil {
		return
	}
	runtime.once.Do(func() { runtime.service.Close(); runtime.stop(); runtime.core.Close(); <-runtime.done })
}

type assistantSelectedSource struct {
	configuration domain.SourceConfiguration
	read          assistantevidence.ReadPorts
}
type assistantResourceResolver struct {
	repository assistantProductRepository
	reader     *assistantevidence.Reader
	sources    map[domain.SourceID]assistantSelectedSource
}

func (resolver *assistantResourceResolver) Resolve(ctx context.Context, actor assistant.Actor, sourceID domain.SourceID, resourceID domain.ResourceID, selected domain.TimeRange) (contextbuilder.Scope, error) {
	if resolver == nil || resolver.repository == nil || resolver.reader == nil || ctx == nil || ctx.Err() != nil || !actor.Valid() || sourceID.Validate() != nil || resourceID.Validate() != nil || selected.ValidateLookback(24*time.Hour) != nil {
		return contextbuilder.Scope{}, assistant.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	approved, ok := resolver.sources[sourceID]
	if !ok {
		return contextbuilder.Scope{}, assistant.ErrDenied
	}
	current, err := resolver.repository.GetSourceConfiguration(ctx, sourceID)
	if err != nil || current.Validate() != nil || !current.Enabled || current.Source.ID != sourceID || current.Source.Provider != approved.configuration.Source.Provider || current.Source.Synthetic != approved.configuration.Source.Synthetic || current.ProfileID != approved.configuration.ProfileID {
		return contextbuilder.Scope{}, assistant.ErrDenied
	}
	source := domain.ProviderRef{Provider: current.Source.Provider, SourceID: sourceID}
	query := domain.ResourceDetailQuery{Source: source, ResourceID: resourceID}
	resource, err := approved.read.GetResource(ctx, query)
	if ctx.Err() != nil || err != nil || resource.ValidateFor(query) != nil || !resource.Status.AllowsData() {
		return contextbuilder.Scope{}, assistant.ErrDenied
	}
	generation, err := resolver.repository.AssistantRestoreGeneration(ctx)
	if ctx.Err() != nil || err != nil || generation < 0 {
		return contextbuilder.Scope{}, assistant.ErrUnavailable
	}
	if err := resolver.reader.AdvanceSourceVersion(source, current.Version); err != nil {
		return contextbuilder.Scope{}, assistant.ErrDenied
	}
	return contextbuilder.Scope{Source: source, ConfigurationVersion: current.Version, PolicyVersion: 1, RestoreGeneration: generation, Range: selected, Resources: []domain.ResourceRef{resource.Resource.Ref}}, nil
}

func assistantProductBinding(profile config.AssistantProfile) (assistantconversation.Binding, error) {
	if profile.Validate() != nil || !profile.Enabled() {
		return assistantconversation.Binding{}, errAssistantStart
	}
	local, _ := profile.Local()
	limits := profile.Limits()
	hash := sha256.New()
	for _, part := range []string{"local-runtime-binding-v1", local.ID, local.Origin, local.ModelAlias, local.ExpectedBuild, local.ExpectedTemplateSHA256, strconv.Itoa(local.ExpectedContextTokens), strconv.FormatInt(local.Generation, 10), strconv.Itoa(limits.OutputTokens), limits.CallTimeout.String()} {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return assistantconversation.Binding{ProfileID: "local_" + hex.EncodeToString(hash.Sum(nil))[:56], ProfileVersion: local.Generation, PolicyVersion: 1, ReservedUnits: int64(limits.InputTokens + limits.OutputTokens)}, nil
}

func reconcileAssistantProduct(ctx context.Context, repository assistantProductRepository, binding assistantconversation.Binding, audit assistant.OperatorAudit) (bool, error) {
	current, err := repository.GetAssistantSettings(ctx)
	if err != nil || !current.Valid() || current.PolicyVersion != binding.PolicyVersion || current.ProfileVersion > binding.ProfileVersion {
		return false, errAssistantStart
	}
	// A retained operator stop survives an identical environment and restart.
	// Selecting new pinned configuration is a separate explicit operator action.
	if current.ProfileID == binding.ProfileID && current.ProfileVersion == binding.ProfileVersion && !current.Enabled {
		return false, nil
	}
	desired := current
	desired.Enabled, desired.PaidEnabled, desired.ProfilePaid = true, false, false
	desired.ProfileID, desired.ProfileVersion = binding.ProfileID, binding.ProfileVersion
	if desired == current {
		return true, nil
	}
	updated, err := repository.ConfigureAssistant(ctx, desired, audit)
	if err != nil || updated.Version != current.Version+1 {
		return false, errAssistantStart
	}
	desired.Version = updated.Version
	if updated != desired {
		return false, errAssistantStart
	}
	return true, nil
}

func buildAssistantProduct(ctx context.Context, settings config.Config, database databaseRuntime, raw, cached []httpapi.ProductProvider, logger *slog.Logger) (*assistantProductRuntime, error) {
	if settings.Assistant.Validate() != nil {
		return nil, errAssistantStart
	}
	if !settings.Assistant.Enabled() {
		return nil, nil
	}
	if ctx == nil || ctx.Err() != nil || settings.GuestDemo || database == nil || logger == nil || len(raw) < 1 || len(raw) > assistantevidence.MaximumSources || len(cached) != len(raw) {
		return nil, errAssistantStart
	}
	repository, ok := database.(assistantProductRepository)
	if !ok {
		return nil, errAssistantStart
	}
	work, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	selected := make(map[domain.SourceID]assistantSelectedSource, len(cached))
	for _, provider := range cached {
		if provider == nil {
			return nil, errAssistantStart
		}
		source := provider.Source()
		read, ok := provider.(assistantevidence.ReadPorts)
		if !ok || source.Validate() != nil {
			return nil, errAssistantStart
		}
		if _, exists := selected[source.ID]; exists {
			return nil, errAssistantStart
		}
		configuration, err := repository.GetSourceConfiguration(work, source.ID)
		if err != nil || configuration.Validate() != nil || configuration.Source.ID != source.ID || configuration.Source.Provider != source.Provider || configuration.Source.Synthetic != source.Synthetic {
			return nil, errAssistantStart
		}
		selected[source.ID] = assistantSelectedSource{configuration: configuration, read: read}
	}
	sources := make([]assistantevidence.Source, 0, len(raw))
	seen := make(map[domain.SourceID]bool, len(raw))
	for _, provider := range raw {
		if provider == nil {
			return nil, errAssistantStart
		}
		source := provider.Source()
		approved, ok := selected[source.ID]
		capture, canCapture := provider.(domain.InventoryCaptureProvider)
		profile := profileFor(source.Provider)
		if source.Provider == domain.ProviderDocker && settings.DockerAccessProfile == config.DockerProfileRestrictedProxy {
			profile = domain.SourceProfileDockerRestrictedProxy
		}
		if source.Provider == domain.ProviderKubernetes && settings.KubernetesAccessProfile == config.KubernetesProfileKubeconfig {
			profile = domain.SourceProfileKubernetesKubeconfig
		}
		if !ok || !canCapture || source.Validate() != nil || seen[source.ID] || approved.configuration.ProfileID != profile || approved.configuration.Source.Provider != source.Provider || approved.configuration.Source.Synthetic != source.Synthetic {
			return nil, errAssistantStart
		}
		seen[source.ID] = true
		reference := domain.ProviderRef{Provider: source.Provider, SourceID: source.ID}
		sources = append(sources, assistantevidence.Source{Reference: reference, ConfigurationVersion: approved.configuration.Version, Read: approved.read, Live: assistantevidence.CaptureAuthority{Source: reference, Provider: capture, Now: time.Now}})
	}
	reader, err := assistantevidence.New(assistantevidence.Options{Gate: assistantevidence.RepositoryGate{Repository: repository}, Sources: sources, Now: time.Now})
	if err != nil {
		return nil, errAssistantStart
	}
	local, _ := settings.Assistant.Local()
	limits := settings.Assistant.Limits()
	backend, err := localprocessor.NewLocal(localprocessor.Profile{ID: local.ID, Origin: local.Origin, ApprovedAddresses: []netip.Addr{local.Address}, ExpectedAlias: local.ModelAlias, ExpectedBuild: local.ExpectedBuild, ExpectedTemplateSHA256: local.ExpectedTemplateSHA256, ExpectedContextTokens: local.ExpectedContextTokens, Generation: uint64(local.Generation)}, localprocessor.Options{Timeout: limits.CallTimeout})
	if err != nil {
		return nil, errAssistantStart
	}
	gateway, err := inference.New(backend, time.Now)
	if err != nil {
		return nil, errAssistantStart
	}
	_, rootKey, err := database.AuthDigestKey()
	defer clear(rootKey)
	if err != nil || len(rootKey) != 32 || !slices.ContainsFunc(rootKey, func(value byte) bool { return value != 0 }) {
		return nil, errAssistantStart
	}
	derive := func(purpose string) []byte {
		mac := hmac.New(sha256.New, rootKey)
		_, _ = mac.Write([]byte(purpose))
		return mac.Sum(nil)
	}
	fingerprintKey, conversationKey := derive("stackplore/helper-fingerprint/v1"), derive("stackplore/helper-conversation/v1")
	defer clear(fingerprintKey)
	defer clear(conversationKey)
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, errAssistantStart
	}
	processID := "helper_" + hex.EncodeToString(random[:])
	lifetime, stop := context.WithCancel(ctx)
	core, err := assistantservice.New(assistantservice.Options{Repository: repository, Gateway: gateway, Authorizer: reader, Collector: reader, Lifetime: lifetime, Now: time.Now, ProcessID: processID, FingerprintKey: fingerprintKey, Workers: 2, PollInterval: time.Second, Heartbeat: time.Second, OutputLimit: limits.OutputTokens})
	if err != nil {
		stop()
		return nil, errAssistantStart
	}
	binding, err := assistantProductBinding(settings.Assistant)
	if err != nil {
		stop()
		core.Close()
		return nil, errAssistantStart
	}
	service, err := assistantconversation.New(assistantconversation.Options{Runtime: core, Repository: repository, Resolver: &assistantResourceResolver{repository: repository, reader: reader, sources: selected}, Binding: binding, Key: conversationKey, Now: time.Now})
	if err != nil {
		stop()
		core.Close()
		return nil, errAssistantStart
	}
	audit := assistant.OperatorAudit{ID: processID, OperatorID: "local_operator", RequestID: processID, At: time.Now().UTC().Truncate(time.Microsecond)}
	enabled, err := reconcileAssistantProduct(work, repository, binding, audit)
	if err != nil || work.Err() != nil {
		service.Close()
		stop()
		core.Close()
		return nil, errAssistantStart
	}
	if !enabled {
		service.Close()
		stop()
		core.Close()
		return nil, nil
	}
	runtime := &assistantProductRuntime{service: service, core: core, stop: stop, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		if err := core.Run(lifetime); err != nil && lifetime.Err() == nil {
			service.Close()
			logger.Error("helper worker stopped", "reason", "worker_unavailable")
		}
	}()
	return runtime, nil
}
