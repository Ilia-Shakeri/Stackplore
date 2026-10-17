package sourcesettings

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const ConfirmationLifetime = 2 * time.Minute

var (
	ErrInvalid              = errors.New("source settings request is invalid")
	ErrVersionConflict      = errors.New("source version changed")
	ErrConfirmationRejected = errors.New("source confirmation was rejected")
	ErrProfileUnavailable   = errors.New("source profile is unavailable")
	ErrStateUnchanged       = errors.New("source state is unchanged")
	ErrUnavailable          = errors.New("source settings are unavailable")
)

type PreviewCommand struct {
	Actor            auth.Principal
	SourceID         domain.SourceID
	ExpectedVersion  int64
	Enabled          bool
	ProfileAvailable bool
	Digest           [32]byte
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

func (command PreviewCommand) Validate() error {
	if command.Actor.Validate() != nil || command.Actor.Role != auth.RoleAdministrator || command.SourceID.Validate() != nil ||
		command.ExpectedVersion < 1 || !validTime(command.CreatedAt) || !validTime(command.ExpiresAt) ||
		command.ExpiresAt.Sub(command.CreatedAt) != ConfirmationLifetime || command.Digest == ([32]byte{}) {
		return ErrInvalid
	}
	return nil
}

type UpdateCommand struct {
	Actor              auth.Principal
	SourceID           domain.SourceID
	ExpectedVersion    int64
	Enabled            bool
	ProfileAvailable   bool
	ConfirmationDigest [32]byte
	Grant              auth.RecentGrantUse
	Audit              domain.AuditEntry
}

func (command UpdateCommand) Validate() error {
	if command.Actor.Validate() != nil || command.Actor.Role != auth.RoleAdministrator || command.SourceID.Validate() != nil ||
		command.ExpectedVersion < 1 || command.ConfirmationDigest == ([32]byte{}) ||
		command.Grant.Actor != command.Actor || command.Grant.Purpose != auth.RecentAuthPurposeSourceUpdate ||
		command.Grant.Digest.Validate() != nil || !validTime(command.Grant.ConsumedAt) ||
		command.Audit.Validate() != nil || command.Audit.Action != "source.update" || command.Audit.Outcome != domain.AuditSuccess ||
		command.Audit.Actor.Kind != domain.AuditActorUser || command.Audit.Actor.ID != string(command.Actor.UserID) ||
		command.Audit.Target.Kind != "infrastructure.source" || command.Audit.Target.ID != string(command.SourceID) ||
		command.Audit.OccurredAt != command.Grant.ConsumedAt || command.Audit.ReasonCode != changeReason(command.Enabled) {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	domain.SourceConfigurationReader
	CreateSourcePreview(context.Context, PreviewCommand) (domain.SourceConfiguration, error)
	CommitSourceUpdate(context.Context, UpdateCommand) (domain.SourceConfiguration, error)
}

type Preview struct {
	Configuration     domain.SourceConfiguration
	RequestedEnabled  bool
	ConfirmationToken string
	ExpiresAt         time.Time
}

func (Preview) String() string { return "[protected source preview]" }

type Service struct {
	repository Repository
	profiles   map[domain.SourceID]domain.SourceProfileID
	now        func() time.Time
}

func New(repository Repository, profiles map[domain.SourceID]domain.SourceProfileID, now func() time.Time) (*Service, error) {
	if repository == nil || now == nil || len(profiles) < 1 || len(profiles) > 100 {
		return nil, ErrInvalid
	}
	copyProfiles := make(map[domain.SourceID]domain.SourceProfileID, len(profiles))
	for id, profile := range profiles {
		if id.Validate() != nil || profile != domain.SourceProfileDemoFixture && profile != domain.SourceProfileDockerSocketDevelopment && profile != domain.SourceProfileDockerRestrictedProxy && profile != domain.SourceProfileKubernetesInCluster && profile != domain.SourceProfileKubernetesKubeconfig {
			return nil, ErrInvalid
		}
		copyProfiles[id] = profile
	}
	return &Service{repository: repository, profiles: copyProfiles, now: now}, nil
}

func (service *Service) ProfileAvailable(configuration domain.SourceConfiguration) bool {
	return service.profiles[configuration.Source.ID] == configuration.ProfileID
}

func (service *Service) List(ctx context.Context, page domain.PageRequest) (domain.SourceConfigurationPage, error) {
	if page.Validate() != nil || page.Limit > 100 {
		return domain.SourceConfigurationPage{}, ErrInvalid
	}
	return service.repository.ListSourceConfigurations(ctx, page)
}

func (service *Service) Preview(ctx context.Context, actor auth.Principal, id domain.SourceID, version int64, enabled bool) (Preview, error) {
	if ctx == nil || ctx.Err() != nil || actor.Validate() != nil || actor.Role != auth.RoleAdministrator || id.Validate() != nil || version < 1 {
		return Preview{}, ErrInvalid
	}
	configuration, err := service.repository.GetSourceConfiguration(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	available := service.ProfileAvailable(configuration)
	if enabled && !available {
		return Preview{}, ErrProfileUnavailable
	}
	at := service.now().UTC().Truncate(time.Microsecond)
	var randomBytes [32]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return Preview{}, ErrUnavailable
	}
	token := base64.RawURLEncoding.EncodeToString(randomBytes[:])
	clear(randomBytes[:])
	digest, _ := confirmationDigest(token)
	command := PreviewCommand{Actor: actor, SourceID: id, ExpectedVersion: version, Enabled: enabled,
		ProfileAvailable: available, Digest: digest, CreatedAt: at, ExpiresAt: at.Add(ConfirmationLifetime)}
	if err := command.Validate(); err != nil {
		return Preview{}, err
	}
	configuration, err = service.repository.CreateSourcePreview(ctx, command)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Configuration: configuration, RequestedEnabled: enabled, ConfirmationToken: token, ExpiresAt: command.ExpiresAt}, nil
}

func (service *Service) Update(ctx context.Context, actor auth.Principal, id domain.SourceID, version int64, enabled bool, token string, grant auth.RecentGrantUse, requestID domain.RequestID) (domain.SourceConfiguration, error) {
	if ctx == nil || ctx.Err() != nil || actor.Validate() != nil || actor.Role != auth.RoleAdministrator || id.Validate() != nil || version < 1 {
		return domain.SourceConfiguration{}, ErrInvalid
	}
	digest, err := confirmationDigest(token)
	if err != nil {
		return domain.SourceConfiguration{}, ErrConfirmationRejected
	}
	configuration, err := service.repository.GetSourceConfiguration(ctx, id)
	if err != nil {
		return domain.SourceConfiguration{}, err
	}
	available := service.ProfileAvailable(configuration)
	if enabled && !available {
		return domain.SourceConfiguration{}, ErrProfileUnavailable
	}
	var randomBytes [18]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return domain.SourceConfiguration{}, ErrUnavailable
	}
	auditID := domain.AuditEntryID("audit_" + base64.RawURLEncoding.EncodeToString(randomBytes[:]))
	clear(randomBytes[:])
	grant.ConsumedAt = service.now().UTC().Truncate(time.Microsecond)
	command := UpdateCommand{Actor: actor, SourceID: id, ExpectedVersion: version, Enabled: enabled,
		ProfileAvailable: available, ConfirmationDigest: digest, Grant: grant,
		Audit: domain.AuditEntry{ID: auditID, OccurredAt: grant.ConsumedAt, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)},
			Action: "source.update", Target: domain.AuditTarget{Kind: "infrastructure.source", ID: string(id)}, Outcome: domain.AuditSuccess,
			RequestID: requestID, ReasonCode: changeReason(enabled)}}
	if err := command.Validate(); err != nil {
		return domain.SourceConfiguration{}, err
	}
	return service.repository.CommitSourceUpdate(ctx, command)
}

func changeReason(enabled bool) domain.ReasonCode {
	if enabled {
		return "source.collection_enabled"
	}
	return "source.collection_disabled"
}

func confirmationDigest(token string) ([32]byte, error) {
	if len(token) != 43 {
		return [32]byte{}, ErrConfirmationRejected
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, ErrConfirmationRejected
	}
	clear(decoded)
	return sha256.Sum256([]byte("stackplore-source-confirmation/v1\x00" + token)), nil
}

func validTime(at time.Time) bool {
	return !at.IsZero() && at.Location() == time.UTC && at.Year() >= 1970 && at.Year() <= 9999 && at.Nanosecond()%int(time.Microsecond) == 0
}
