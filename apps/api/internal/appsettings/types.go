package appsettings

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const ConfirmationLifetime = 2 * time.Minute

var (
	ErrInvalid      = errors.New("application settings request invalid")
	ErrForbidden    = errors.New("application settings forbidden")
	ErrVersion      = errors.New("application settings version changed")
	ErrConfirmation = errors.New("application settings confirmation rejected")
	ErrBackup       = errors.New("backup acknowledgement required")
	ErrUnavailable  = errors.New("application settings unavailable")
)

type PreviewKind string

const (
	PreviewSettings PreviewKind = "settings"
	PreviewCleanup  PreviewKind = "cleanup"
)

type PreviewCommand struct {
	Actor           auth.Principal
	Kind            PreviewKind
	ExpectedVersion int64
	Values          domain.ApplicationValues
	Classes         []domain.HistoryClass
	Digest          [32]byte
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

func (command PreviewCommand) Validate() error {
	if command.Actor.Validate() != nil || command.Actor.Role != auth.RoleAdministrator || command.ExpectedVersion < 1 || command.Digest == ([32]byte{}) || !validTime(command.CreatedAt) || !validTime(command.ExpiresAt) || command.ExpiresAt.Sub(command.CreatedAt) != ConfirmationLifetime {
		return ErrInvalid
	}
	switch command.Kind {
	case PreviewSettings:
		if command.Values.Validate() != nil || len(command.Classes) != 0 {
			return ErrInvalid
		}
	case PreviewCleanup:
		if command.Values != (domain.ApplicationValues{}) || domain.ValidateHistoryClasses(command.Classes) != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type StoredPreview struct {
	Settings  domain.ApplicationSettingsRecord
	Impact    domain.HistoryImpact
	ExpiresAt time.Time
}

type Preview struct {
	Settings          domain.ApplicationSettingsRecord
	Requested         domain.ApplicationValues
	Impact            domain.HistoryImpact
	ConfirmationToken string
	ExpiresAt         time.Time
}

func (Preview) String() string { return "[protected settings preview]" }

type CleanupPreview struct {
	PolicyVersion     int64
	Impact            domain.HistoryImpact
	ConfirmationToken string
	ExpiresAt         time.Time
}

func (CleanupPreview) String() string { return "[protected cleanup preview]" }

type CommitCommand struct {
	Actor              auth.Principal
	ExpectedVersion    int64
	Values             domain.ApplicationValues
	ConfirmationDigest [32]byte
	BackupAcknowledged bool
	Grant              auth.RecentGrantUse
	Audit              domain.AuditEntry
}

func (command CommitCommand) Validate(purpose auth.RecentAuthPurpose, action domain.AuditAction) error {
	if command.Actor.Validate() != nil || command.Actor.Role != auth.RoleAdministrator || command.ExpectedVersion < 1 || command.ConfirmationDigest == ([32]byte{}) ||
		command.Grant.Actor != command.Actor || command.Grant.Purpose != purpose || command.Grant.Digest.Validate() != nil || !validTime(command.Grant.ConsumedAt) ||
		command.Audit.Validate() != nil || command.Audit.Action != action || command.Audit.Outcome != domain.AuditSuccess || command.Audit.Actor.Kind != domain.AuditActorUser || command.Audit.Actor.ID != string(command.Actor.UserID) || command.Audit.Target.Kind != "application.settings" || command.Audit.Target.ID != "application" || command.Audit.OccurredAt != command.Grant.ConsumedAt {
		return ErrInvalid
	}
	return nil
}

type CleanupResult struct {
	RemovedRows   int64                  `json:"removed_rows"`
	RemainingRows bool                   `json:"remaining_rows"`
	Scope         []domain.HistoryClass  `json:"scope"`
	Cutoffs       []domain.HistoryCutoff `json:"cutoffs"`
	PolicyVersion int64                  `json:"policy_version"`
}

type ExportSource struct {
	Provider            domain.ProviderKind    `json:"provider"`
	Name                string                 `json:"name"`
	Enabled             bool                   `json:"enabled"`
	Profile             domain.SourceProfileID `json:"profile"`
	CredentialReference string                 `json:"credential_reference"`
	Host                string                 `json:"host"`
}

type ExportIntegration struct {
	Kind                string `json:"kind"`
	Enabled             bool   `json:"enabled"`
	Target              string `json:"target"`
	CredentialReference string `json:"credential_reference"`
}

type ConfigurationExport struct {
	ApplicationVersion       string                   `json:"application_version"`
	Settings                 domain.ApplicationValues `json:"settings"`
	Sources                  []ExportSource           `json:"sources"`
	NotificationIntegrations []ExportIntegration      `json:"notification_integrations"`
}

type ExportCommand struct {
	Actor                    auth.Principal
	Grant                    auth.RecentGrantUse
	Audit                    domain.AuditEntry
	ApplicationVersion       string
	NotificationIntegrations []ExportIntegration
}

type Repository interface {
	ReadApplicationSettings(context.Context) (domain.ApplicationSettingsRecord, error)
	ReadAuditPolicyStatus(context.Context) (domain.AuditPolicyStatus, error)
	ConfigureAuditLifecyclePolicy(context.Context, domain.AuditLifecyclePolicy) error
	PruneAuditHistory(context.Context, int) (int64, error)
	CreateApplicationPreview(context.Context, PreviewCommand) (StoredPreview, error)
	CommitApplicationSettings(context.Context, CommitCommand) (domain.ApplicationSettingsRecord, error)
	CommitHistoryCleanup(context.Context, CommitCommand) (CleanupResult, error)
	CommitConfigurationExport(context.Context, ExportCommand) (ConfigurationExport, error)
}

func validTime(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC && value.UnixMicro() >= 0 && value.Nanosecond()%int(time.Microsecond) == 0
}
