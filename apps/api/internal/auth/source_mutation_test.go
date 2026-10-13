package auth

import (
	"errors"
	"testing"
)

func TestPrepareSourceMutationKeepsGrantForAtomicRepositoryCommit(t *testing.T) {
	fixture := setupServiceFixture(t)
	session := fixture.login(t, "request_source_login")
	grant := fixture.recentGrant(t, session, RecentAuthPurposeSourceUpdate, "request_source_grant")
	input := RecentAuthorizationInput{SessionToken: encodeTestToken(t, session.Token), CSRFProof: encodeTestToken(t, session.Context.CSRFProof),
		RecentProof: encodeTestToken(t, grant.Proof), Purpose: RecentAuthPurposeSourceUpdate, RequestID: "request_source_prepare"}
	identity, use, err := fixture.service.PrepareSourceMutation(fixture.ctx, input)
	if err != nil || use.Actor != identity.Principal || use.Purpose != RecentAuthPurposeSourceUpdate || use.Digest.Validate() != nil {
		t.Fatal("source grant preparation failed")
	}
	if _, _, err := fixture.service.PrepareSourceMutation(fixture.ctx, input); err != nil {
		t.Fatal("preparation prematurely consumed grant")
	}
	input.Purpose = RecentAuthPurposeNotificationTest
	if _, _, err := fixture.service.PrepareSourceMutation(fixture.ctx, input); !errors.Is(err, ErrInvalidServiceInput) {
		t.Fatal("source preparation accepted another purpose")
	}
	input.Purpose = RecentAuthPurposeSourceUpdate
	input.CSRFProof = ""
	if _, _, err := fixture.service.PrepareSourceMutation(fixture.ctx, input); !errors.Is(err, ErrCSRFRejected) {
		t.Fatal("source preparation bypassed CSRF")
	}
}
