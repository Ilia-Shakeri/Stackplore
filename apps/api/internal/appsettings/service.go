package appsettings

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type Service struct {
	repository   Repository
	key          [32]byte
	now          func() time.Time
	version      string
	integrations []ExportIntegration
}

func New(repository Repository, key []byte, now func() time.Time, version string, integrations []ExportIntegration) (*Service, error) {
	if repository == nil || len(key) != 32 || now == nil || len(version) < 1 || len(version) > 64 || len(integrations) > 2 {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	for _, integration := range integrations {
		if (integration.Kind != "webhook" && integration.Kind != "matrix") || seen[integration.Kind] || integration.Target != "[masked]" || integration.CredentialReference != "[masked]" {
			return nil, ErrInvalid
		}
		seen[integration.Kind] = true
	}
	service := &Service{repository: repository, now: now, version: version, integrations: append([]ExportIntegration{}, integrations...)}
	copy(service.key[:], key)
	return service, nil
}

func (service *Service) Get(ctx context.Context) (domain.ApplicationSettingsRecord, error) {
	return service.repository.ReadApplicationSettings(ctx)
}
func (service *Service) AuditPolicy(ctx context.Context) (domain.AuditPolicyStatus, error) {
	return service.repository.ReadAuditPolicyStatus(ctx)
}
func (service *Service) ConfigureAuditPolicy(ctx context.Context, policy domain.AuditLifecyclePolicy) error {
	if ctx == nil || policy.Validate() != nil {
		return ErrInvalid
	}
	return service.repository.ConfigureAuditLifecyclePolicy(ctx, policy)
}
func (service *Service) PruneAuditHistory(ctx context.Context, limit int) (int64, error) {
	return service.repository.PruneAuditHistory(ctx, limit)
}
func (service *Service) Version() string { return service.version }

func (service *Service) Preview(ctx context.Context, actor auth.Principal, version int64, values domain.ApplicationValues) (Preview, error) {
	command, proof, err := service.previewCommand(ctx, actor, version, PreviewSettings, values, nil)
	if err != nil {
		return Preview{}, err
	}
	stored, err := service.repository.CreateApplicationPreview(ctx, command)
	if err != nil {
		return Preview{}, err
	}
	if stored.Settings.Validate() != nil || stored.Impact.Validate() != nil || stored.ExpiresAt != command.ExpiresAt {
		return Preview{}, ErrUnavailable
	}
	return Preview{Settings: stored.Settings, Requested: values, Impact: stored.Impact, ConfirmationToken: proof, ExpiresAt: stored.ExpiresAt}, nil
}

func (service *Service) PreviewCleanup(ctx context.Context, actor auth.Principal, version int64, classes []domain.HistoryClass) (CleanupPreview, error) {
	command, proof, err := service.previewCommand(ctx, actor, version, PreviewCleanup, domain.ApplicationValues{}, classes)
	if err != nil {
		return CleanupPreview{}, err
	}
	stored, err := service.repository.CreateApplicationPreview(ctx, command)
	if err != nil {
		return CleanupPreview{}, err
	}
	if stored.Settings.Validate() != nil || stored.Impact.Validate() != nil || len(stored.Impact.Classes) != len(classes) || stored.ExpiresAt != command.ExpiresAt {
		return CleanupPreview{}, ErrUnavailable
	}
	return CleanupPreview{PolicyVersion: stored.Settings.Version, Impact: stored.Impact, ConfirmationToken: proof, ExpiresAt: stored.ExpiresAt}, nil
}

func (service *Service) previewCommand(ctx context.Context, actor auth.Principal, version int64, kind PreviewKind, values domain.ApplicationValues, classes []domain.HistoryClass) (PreviewCommand, string, error) {
	if ctx == nil || ctx.Err() != nil || actor.Validate() != nil || actor.Role != auth.RoleAdministrator {
		return PreviewCommand{}, "", ErrForbidden
	}
	proof, err := auth.GenerateOpaqueToken()
	if err != nil {
		return PreviewCommand{}, "", ErrUnavailable
	}
	encoded, err := proof.Encode()
	if err != nil {
		return PreviewCommand{}, "", ErrUnavailable
	}
	now := service.now().UTC().Truncate(time.Microsecond)
	command := PreviewCommand{Actor: actor, Kind: kind, ExpectedVersion: version, Values: values, Classes: append([]domain.HistoryClass(nil), classes...), Digest: service.digest(encoded), CreatedAt: now, ExpiresAt: now.Add(ConfirmationLifetime)}
	if command.Validate() != nil {
		return PreviewCommand{}, "", ErrInvalid
	}
	return command, encoded, nil
}

func (service *Service) Update(ctx context.Context, actor auth.Principal, version int64, values domain.ApplicationValues, proof string, backup bool, grant auth.RecentGrantUse, requestID domain.RequestID) (domain.ApplicationSettingsRecord, error) {
	if values.Validate() != nil {
		return domain.ApplicationSettingsRecord{}, ErrInvalid
	}
	command, err := service.commitCommand(ctx, actor, version, proof, backup, grant, requestID, auth.RecentAuthPurposeSettingsUpdate, "settings.update")
	if err != nil {
		return domain.ApplicationSettingsRecord{}, err
	}
	command.Values = values
	return service.repository.CommitApplicationSettings(ctx, command)
}

func (service *Service) Cleanup(ctx context.Context, actor auth.Principal, version int64, proof string, backup bool, grant auth.RecentGrantUse, requestID domain.RequestID) (CleanupResult, error) {
	command, err := service.commitCommand(ctx, actor, version, proof, backup, grant, requestID, auth.RecentAuthPurposeDataCleanup, "data.cleanup")
	if err != nil {
		return CleanupResult{}, err
	}
	return service.repository.CommitHistoryCleanup(ctx, command)
}

func (service *Service) commitCommand(ctx context.Context, actor auth.Principal, version int64, proof string, backup bool, grant auth.RecentGrantUse, requestID domain.RequestID, purpose auth.RecentAuthPurpose, action domain.AuditAction) (CommitCommand, error) {
	if ctx == nil || ctx.Err() != nil || requestID.Validate() != nil {
		return CommitCommand{}, ErrInvalid
	}
	parsed, err := auth.ParseOpaqueToken(proof)
	if err != nil {
		return CommitCommand{}, ErrConfirmation
	}
	encoded, err := parsed.Encode()
	if err != nil {
		return CommitCommand{}, ErrConfirmation
	}
	audit, err := newAudit(actor, grant, requestID, action)
	if err != nil {
		return CommitCommand{}, err
	}
	command := CommitCommand{Actor: actor, ExpectedVersion: version, ConfirmationDigest: service.digest(encoded), BackupAcknowledged: backup, Grant: grant, Audit: audit}
	if command.Validate(purpose, action) != nil {
		return CommitCommand{}, ErrInvalid
	}
	return command, nil
}

func (service *Service) Export(ctx context.Context, actor auth.Principal, grant auth.RecentGrantUse, requestID domain.RequestID) (ConfigurationExport, error) {
	if ctx == nil || ctx.Err() != nil || actor.Validate() != nil || actor.Role != auth.RoleAdministrator || grant.Actor != actor || grant.Purpose != auth.RecentAuthPurposeConfigurationExport || grant.Digest.Validate() != nil || !validTime(grant.ConsumedAt) || requestID.Validate() != nil {
		return ConfigurationExport{}, ErrForbidden
	}
	audit, err := newAudit(actor, grant, requestID, "configuration.export")
	if err != nil {
		return ConfigurationExport{}, err
	}
	return service.repository.CommitConfigurationExport(ctx, ExportCommand{Actor: actor, Grant: grant, Audit: audit, ApplicationVersion: service.version, NotificationIntegrations: append([]ExportIntegration{}, service.integrations...)})
}

func (service *Service) digest(proof string) [32]byte {
	mac := hmac.New(sha256.New, service.key[:])
	_, _ = mac.Write([]byte("stackplore/application-confirmation/v1\x00"))
	_, _ = mac.Write([]byte(proof))
	var digest [32]byte
	copy(digest[:], mac.Sum(nil))
	return digest
}

func newAudit(actor auth.Principal, grant auth.RecentGrantUse, requestID domain.RequestID, action domain.AuditAction) (domain.AuditEntry, error) {
	token, err := auth.GenerateOpaqueToken()
	if err != nil {
		return domain.AuditEntry{}, ErrUnavailable
	}
	encoded, err := token.Encode()
	if err != nil {
		return domain.AuditEntry{}, ErrUnavailable
	}
	id := domain.AuditEntryID("audit_" + encoded)
	audit := domain.AuditEntry{ID: id, OccurredAt: grant.ConsumedAt, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, Action: action, Target: domain.AuditTarget{Kind: "application.settings", ID: "application"}, Outcome: domain.AuditSuccess, RequestID: requestID, ReasonCode: domain.ReasonCode(string(action) + "_committed")}
	if audit.Validate() != nil {
		return domain.AuditEntry{}, ErrInvalid
	}
	return audit, nil
}
