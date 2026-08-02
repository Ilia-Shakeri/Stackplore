package auth

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestOpaqueTokenUsesCanonicalFixedEncoding(t *testing.T) {
	token := mustCoreToken(t, 7)
	encoded, err := token.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != OpaqueTokenEncodedSize || bytes.ContainsAny([]byte(encoded), "+/=") {
		t.Fatalf("encoded proof has wrong form: length %d", len(encoded))
	}
	parsed, err := ParseOpaqueToken(encoded)
	if err != nil || !parsed.Equal(token) {
		t.Fatalf("ParseOpaqueToken() = (%v, %v)", parsed, err)
	}
	if got := fmt.Sprint(token); got != "[protected proof]" {
		t.Fatalf("formatted proof = %q", got)
	}
	if got := fmt.Sprintf("%#v", token); got != "auth.OpaqueToken{}" {
		t.Fatalf("Go-syntax proof = %q", got)
	}

	for _, invalid := range []string{"", encoded + "=", encoded[:42], encoded[:42] + "+"} {
		if _, err := ParseOpaqueToken(invalid); !errors.Is(err, ErrInvalidOpaqueToken) {
			t.Fatalf("ParseOpaqueToken(%q) error = %v", invalid, err)
		}
	}
	if _, err := (OpaqueToken{}).Encode(); !errors.Is(err, ErrInvalidOpaqueToken) {
		t.Fatalf("zero Encode() error = %v", err)
	}
	if _, err := generateOpaqueToken(bytes.NewReader(make([]byte, OpaqueTokenBytes-1))); !errors.Is(err, ErrTokenGenerationFailed) {
		t.Fatalf("short random source error = %v", err)
	}
}

func TestTypedDigestsUseDomainSeparationAndCopies(t *testing.T) {
	key := bytes.Repeat([]byte{3}, DigestKeyBytes)
	token := mustCoreToken(t, 4)
	sessionDigest, err := DigestSessionToken(key, token)
	if err != nil {
		t.Fatal(err)
	}
	preauthDigest, err := DigestPreauthCookie(key, token)
	if err != nil {
		t.Fatal(err)
	}
	sessionBytes, err := sessionDigest.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	preauthBytes, err := preauthDigest.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(sessionBytes, preauthBytes) {
		t.Fatal("different proof classes produced one digest")
	}

	parsed, err := ParseSessionTokenDigest(sessionBytes)
	if err != nil || !parsed.Equal(sessionDigest) {
		t.Fatalf("parsed digest mismatch: %v", err)
	}
	sessionBytes[0] ^= 0xff
	untouched, err := sessionDigest.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(sessionBytes, untouched) {
		t.Fatal("digest bytes did not return a copy")
	}

	otherKeyDigest, err := DigestSessionToken(bytes.Repeat([]byte{5}, DigestKeyBytes), token)
	if err != nil {
		t.Fatal(err)
	}
	if sessionDigest.Equal(otherKeyDigest) {
		t.Fatal("different keys produced an equal typed digest")
	}
	if _, err := DigestSessionToken(key[:31], token); !errors.Is(err, ErrInvalidDigestKey) {
		t.Fatalf("short key error = %v", err)
	}
	if _, err := ParseSessionTokenDigest(make([]byte, DigestBytes-1)); !errors.Is(err, ErrInvalidTypedDigest) {
		t.Fatalf("short digest error = %v", err)
	}
	if !errors.Is((SessionTokenDigest{}).Validate(), ErrInvalidTypedDigest) {
		t.Fatal("zero typed digest passed validation")
	}
}

func TestCSRFProofIsDeterministicAndSessionBound(t *testing.T) {
	key := bytes.Repeat([]byte{8}, DigestKeyBytes)
	firstSession := mustCoreToken(t, 1)
	secondSession := mustCoreToken(t, 2)
	first, err := DeriveCSRFProof(key, firstSession)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := DeriveCSRFProof(key, firstSession)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DeriveCSRFProof(key, secondSession)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Equal(repeated) || first.Equal(firstSession) || first.Equal(second) {
		t.Fatal("derived CSRF proof lost deterministic session binding")
	}
	digest, err := DigestCSRFProof(key, first)
	if err != nil || digest.Validate() != nil {
		t.Fatalf("CSRF digest error = %v", err)
	}
}
