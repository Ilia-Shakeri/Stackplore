package auth

import "context"

// PrepareSourceMutation verifies the administrator and CSRF proof, then digests
// the recent proof. The source repository must consume the grant, recheck the
// live principal, and commit the exact source change in one transaction.
func (service *Service) PrepareSourceMutation(ctx context.Context, input RecentAuthorizationInput) (Authentication, RecentGrantUse, error) {
	if input.Purpose != RecentAuthPurposeSourceUpdate || input.RequestID.Validate() != nil {
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
