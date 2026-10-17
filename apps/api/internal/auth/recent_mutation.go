package auth

import "context"

// PrepareRecentMutation checks the live administrator and CSRF proof. The
// receiving use case must consume the purpose-bound grant in its own atomic
// transaction together with confirmation, state change and durable audit.
func (service *Service) PrepareRecentMutation(ctx context.Context, input RecentAuthorizationInput) (Authentication, RecentGrantUse, error) {
	if input.RequestID.Validate() != nil {
		return Authentication{}, RecentGrantUse{}, ErrInvalidServiceInput
	}
	switch input.Purpose {
	case RecentAuthPurposeOperationExecute, RecentAuthPurposeSettingsUpdate,
		RecentAuthPurposeConfigurationExport, RecentAuthPurposeDataCleanup:
	default:
		return Authentication{}, RecentGrantUse{}, ErrInvalidServiceInput
	}
	identity, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return Authentication{}, RecentGrantUse{}, err
	}
	at, err := service.currentTime()
	if err != nil {
		return Authentication{}, RecentGrantUse{}, err
	}
	grant, err := service.recentGrantUse(identity.Principal, input.Purpose, input.RecentProof, at)
	return identity, grant, err
}
