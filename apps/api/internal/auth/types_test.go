package auth

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestCanonicalLoginName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "lowercase", value: "operator", want: "operator"},
		{name: "mixed case", value: "On.Call-01", want: "on.call-01"},
		{name: "too short", value: "ab"},
		{name: "leading punctuation", value: ".operator"},
		{name: "trailing punctuation", value: "operator_"},
		{name: "space", value: "on call"},
		{name: "non ASCII", value: "operatör"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CanonicalLoginName(test.value)
			if test.want == "" {
				if !errors.Is(err, ErrInvalidLoginName) || got != "" {
					t.Fatalf("CanonicalLoginName() = (%q, %v)", got, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("CanonicalLoginName() = (%q, %v); want %q", got, err, test.want)
			}
		})
	}
}

func TestAuthenticationModelsValidateBoundState(t *testing.T) {
	created := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	token := mustCoreToken(t, 1)
	csrf, err := DeriveCSRFProof(bytes.Repeat([]byte{9}, DigestKeyBytes), token)
	if err != nil {
		t.Fatal(err)
	}
	sessionDigest, err := DigestSessionToken(bytes.Repeat([]byte{9}, DigestKeyBytes), token)
	if err != nil {
		t.Fatal(err)
	}
	csrfDigest, err := DigestCSRFProof(bytes.Repeat([]byte{9}, DigestKeyBytes), csrf)
	if err != nil {
		t.Fatal(err)
	}
	preauthCookie, err := DigestPreauthCookie(bytes.Repeat([]byte{9}, DigestKeyBytes), token)
	if err != nil {
		t.Fatal(err)
	}
	preauthProof, err := DigestPreauthProof(bytes.Repeat([]byte{9}, DigestKeyBytes), csrf)
	if err != nil {
		t.Fatal(err)
	}
	recentDigest, err := DigestRecentAuthProof(bytes.Repeat([]byte{9}, DigestKeyBytes), csrf)
	if err != nil {
		t.Fatal(err)
	}

	user := User{
		ID:                 "usr_demo_01",
		LoginName:          "Operator",
		CanonicalLoginName: "operator",
		DisplayName:        "Operations",
		Role:               RoleAdministrator,
		Enabled:            true,
		AuthRevision:       1,
		CreatedAt:          created,
		UpdatedAt:          created,
		Version:            1,
	}
	if err := user.Validate(); err != nil {
		t.Fatalf("User.Validate() error = %v", err)
	}

	session := Session{
		ID:                "ses_demo_01",
		UserID:            user.ID,
		TokenDigest:       sessionDigest,
		TokenDigestKeyID:  "key_demo_01",
		CSRFDigest:        csrfDigest,
		CSRFDigestKeyID:   "key_demo_01",
		UserAuthRevision:  1,
		CreatedAt:         created,
		LastSeenAt:        created,
		IdleExpiresAt:     created.Add(30 * time.Minute),
		AbsoluteExpiresAt: created.Add(12 * time.Hour),
	}
	if err := session.Validate(); err != nil {
		t.Fatalf("Session.Validate() error = %v", err)
	}

	preauth := Preauth{
		ID:           "pre_demo_01",
		Purpose:      PreauthPurposeLogin,
		CookieDigest: preauthCookie,
		ProofDigest:  preauthProof,
		DigestKeyID:  "key_demo_01",
		CreatedAt:    created,
		ExpiresAt:    created.Add(5 * time.Minute),
	}
	if err := preauth.Validate(); err != nil {
		t.Fatalf("Preauth.Validate() error = %v", err)
	}

	grant := RecentGrant{
		ID:          "gra_demo_01",
		UserID:      user.ID,
		SessionID:   session.ID,
		Purpose:     RecentAuthPurposeUserUpdate,
		Digest:      recentDigest,
		DigestKeyID: "key_demo_01",
		CreatedAt:   created,
		ExpiresAt:   created.Add(5 * time.Minute),
	}
	if err := grant.Validate(); err != nil {
		t.Fatalf("RecentGrant.Validate() error = %v", err)
	}

	principal := Principal{UserID: user.ID, SessionID: session.ID, Role: user.Role, AuthRevision: 1}
	if err := principal.Validate(); err != nil {
		t.Fatalf("Principal.Validate() error = %v", err)
	}

	user.CanonicalLoginName = "Operator"
	if !errors.Is(user.Validate(), ErrInvalidAuthState) {
		t.Fatal("non-canonical stored login passed validation")
	}
	session.IdleExpiresAt = created.Add(-time.Second)
	if !errors.Is(session.Validate(), ErrInvalidAuthState) {
		t.Fatal("invalid idle expiry passed validation")
	}
	grant.Purpose = "source.create"
	if !errors.Is(grant.Validate(), ErrInvalidAuthState) {
		t.Fatal("later-phase grant purpose passed validation")
	}
}

func TestDisabledUserLifecycleValidation(t *testing.T) {
	created := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	disabled := created.Add(time.Minute)
	deleted := disabled.Add(time.Minute)
	user := User{
		ID:                 "usr_demo_01",
		LoginName:          "operator",
		CanonicalLoginName: "operator",
		DisplayName:        "Operations",
		Role:               RoleAdministrator,
		AuthRevision:       2,
		CreatedAt:          created,
		UpdatedAt:          deleted,
		DisabledAt:         &disabled,
		DeletedAt:          &deleted,
		Version:            3,
	}
	if err := user.Validate(); err != nil {
		t.Fatalf("disabled deleted user error = %v", err)
	}
	user.Enabled = true
	if !errors.Is(user.Validate(), ErrInvalidAuthState) {
		t.Fatal("enabled deleted user passed validation")
	}
}

func mustCoreToken(t *testing.T, fill byte) OpaqueToken {
	t.Helper()
	token, err := generateOpaqueToken(bytes.NewReader(bytes.Repeat([]byte{fill}, OpaqueTokenBytes)))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
