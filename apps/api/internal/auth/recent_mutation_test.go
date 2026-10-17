package auth

import (
	"errors"
	"testing"
)

func TestPrepareRecentMutationPreservesGrantForEachAtomicUseCase(t *testing.T) {
	fixture := setupServiceFixture(t)
	session := fixture.login(t, "request_guarded_login")
	for _, purpose := range []RecentAuthPurpose{RecentAuthPurposeOperationExecute, RecentAuthPurposeSettingsUpdate, RecentAuthPurposeConfigurationExport, RecentAuthPurposeDataCleanup} {
		grant := fixture.recentGrant(t, session, purpose, "request_guarded_grant")
		input := RecentAuthorizationInput{SessionToken: encodeTestToken(t, session.Token), CSRFProof: encodeTestToken(t, session.Context.CSRFProof),
			RecentProof: encodeTestToken(t, grant.Proof), Purpose: purpose, RequestID: "request_guarded_prepare"}
		for range 2 {
			identity, use, err := fixture.service.PrepareRecentMutation(fixture.ctx, input)
			if err != nil || use.Actor != identity.Principal || use.Purpose != purpose || use.Digest.Validate() != nil {
				t.Fatal("guard preparation consumed or misbound grant")
			}
		}
		input.CSRFProof = ""
		if _, _, err := fixture.service.PrepareRecentMutation(fixture.ctx, input); !errors.Is(err, ErrCSRFRejected) {
			t.Fatal("guard preparation bypassed CSRF")
		}
		input.Purpose = RecentAuthPurposeUserDelete
		if _, _, err := fixture.service.PrepareRecentMutation(fixture.ctx, input); !errors.Is(err, ErrInvalidServiceInput) {
			t.Fatal("guard preparation admitted a separate user lifecycle")
		}
	}
}
