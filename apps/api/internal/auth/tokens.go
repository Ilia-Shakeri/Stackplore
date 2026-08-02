package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
)

const (
	OpaqueTokenBytes       = 32
	OpaqueTokenEncodedSize = 43
	DigestKeyBytes         = 32
	DigestBytes            = sha256.Size
)

var (
	ErrInvalidOpaqueToken    = errors.New("opaque authentication proof is invalid")
	ErrTokenGenerationFailed = errors.New("authentication proof generation failed")
	ErrInvalidDigestKey      = errors.New("authentication digest key is invalid")
	ErrInvalidTypedDigest    = errors.New("authentication digest is invalid")
)

type OpaqueToken struct {
	value [OpaqueTokenBytes]byte
	valid bool
}

// GenerateOpaqueToken creates one 256-bit proof from the system random source.
func GenerateOpaqueToken() (OpaqueToken, error) {
	return generateOpaqueToken(rand.Reader)
}

func generateOpaqueToken(source io.Reader) (OpaqueToken, error) {
	if source == nil {
		return OpaqueToken{}, ErrTokenGenerationFailed
	}
	var value [OpaqueTokenBytes]byte
	if _, err := io.ReadFull(source, value[:]); err != nil {
		clear(value[:])
		return OpaqueToken{}, ErrTokenGenerationFailed
	}
	return OpaqueToken{value: value, valid: true}, nil
}

// ParseOpaqueToken accepts only canonical unpadded URL-safe base64 for 32 bytes.
func ParseOpaqueToken(encoded string) (OpaqueToken, error) {
	if len(encoded) != OpaqueTokenEncodedSize {
		return OpaqueToken{}, ErrInvalidOpaqueToken
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(decoded) != OpaqueTokenBytes || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		clear(decoded)
		return OpaqueToken{}, ErrInvalidOpaqueToken
	}
	var value [OpaqueTokenBytes]byte
	copy(value[:], decoded)
	clear(decoded)
	return OpaqueToken{value: value, valid: true}, nil
}

// Encode reveals the proof only at the exact browser or persistence boundary.
func (token OpaqueToken) Encode() (string, error) {
	if !token.valid {
		return "", ErrInvalidOpaqueToken
	}
	return base64.RawURLEncoding.EncodeToString(token.value[:]), nil
}

func (token OpaqueToken) Equal(other OpaqueToken) bool {
	return token.valid && other.valid && subtle.ConstantTimeCompare(token.value[:], other.value[:]) == 1
}

func (token OpaqueToken) String() string {
	return "[protected proof]"
}

func (token OpaqueToken) GoString() string {
	return "auth.OpaqueToken{}"
}

type SessionTokenDigest struct {
	sum   [DigestBytes]byte
	valid bool
}

type CSRFDigest struct {
	sum   [DigestBytes]byte
	valid bool
}

type PreauthCookieDigest struct {
	sum   [DigestBytes]byte
	valid bool
}

type PreauthProofDigest struct {
	sum   [DigestBytes]byte
	valid bool
}

type RecentAuthDigest struct {
	sum   [DigestBytes]byte
	valid bool
}

func DigestSessionToken(key []byte, token OpaqueToken) (SessionTokenDigest, error) {
	sum, err := digestOpaqueToken(key, "stackplore/session-token/v1", token)
	if err != nil {
		return SessionTokenDigest{}, err
	}
	return SessionTokenDigest{sum: sum, valid: true}, nil
}

func DigestCSRFProof(key []byte, proof OpaqueToken) (CSRFDigest, error) {
	sum, err := digestOpaqueToken(key, "stackplore/csrf-proof-digest/v1", proof)
	if err != nil {
		return CSRFDigest{}, err
	}
	return CSRFDigest{sum: sum, valid: true}, nil
}

func DigestPreauthCookie(key []byte, token OpaqueToken) (PreauthCookieDigest, error) {
	sum, err := digestOpaqueToken(key, "stackplore/preauth-cookie/v1", token)
	if err != nil {
		return PreauthCookieDigest{}, err
	}
	return PreauthCookieDigest{sum: sum, valid: true}, nil
}

func DigestPreauthProof(key []byte, proof OpaqueToken) (PreauthProofDigest, error) {
	sum, err := digestOpaqueToken(key, "stackplore/preauth-proof/v1", proof)
	if err != nil {
		return PreauthProofDigest{}, err
	}
	return PreauthProofDigest{sum: sum, valid: true}, nil
}

func DigestRecentAuthProof(key []byte, proof OpaqueToken) (RecentAuthDigest, error) {
	sum, err := digestOpaqueToken(key, "stackplore/recent-auth-proof/v1", proof)
	if err != nil {
		return RecentAuthDigest{}, err
	}
	return RecentAuthDigest{sum: sum, valid: true}, nil
}

// DeriveCSRFProof binds the browser proof to one opaque session without exposing the digest key.
func DeriveCSRFProof(key []byte, sessionToken OpaqueToken) (OpaqueToken, error) {
	sum, err := digestOpaqueToken(key, "stackplore/csrf-proof/v1", sessionToken)
	if err != nil {
		return OpaqueToken{}, err
	}
	var value [OpaqueTokenBytes]byte
	copy(value[:], sum[:])
	clear(sum[:])
	return OpaqueToken{value: value, valid: true}, nil
}

func ParseSessionTokenDigest(value []byte) (SessionTokenDigest, error) {
	sum, err := parseTypedDigest(value)
	if err != nil {
		return SessionTokenDigest{}, err
	}
	return SessionTokenDigest{sum: sum, valid: true}, nil
}

func ParseCSRFDigest(value []byte) (CSRFDigest, error) {
	sum, err := parseTypedDigest(value)
	if err != nil {
		return CSRFDigest{}, err
	}
	return CSRFDigest{sum: sum, valid: true}, nil
}

func ParsePreauthCookieDigest(value []byte) (PreauthCookieDigest, error) {
	sum, err := parseTypedDigest(value)
	if err != nil {
		return PreauthCookieDigest{}, err
	}
	return PreauthCookieDigest{sum: sum, valid: true}, nil
}

func ParsePreauthProofDigest(value []byte) (PreauthProofDigest, error) {
	sum, err := parseTypedDigest(value)
	if err != nil {
		return PreauthProofDigest{}, err
	}
	return PreauthProofDigest{sum: sum, valid: true}, nil
}

func ParseRecentAuthDigest(value []byte) (RecentAuthDigest, error) {
	sum, err := parseTypedDigest(value)
	if err != nil {
		return RecentAuthDigest{}, err
	}
	return RecentAuthDigest{sum: sum, valid: true}, nil
}

func (digest SessionTokenDigest) Validate() error { return validateTypedDigest(digest.valid) }
func (digest CSRFDigest) Validate() error         { return validateTypedDigest(digest.valid) }
func (digest PreauthCookieDigest) Validate() error {
	return validateTypedDigest(digest.valid)
}
func (digest PreauthProofDigest) Validate() error { return validateTypedDigest(digest.valid) }
func (digest RecentAuthDigest) Validate() error   { return validateTypedDigest(digest.valid) }

func (digest SessionTokenDigest) Bytes() ([]byte, error) {
	return typedDigestBytes(digest.sum, digest.valid)
}
func (digest CSRFDigest) Bytes() ([]byte, error) { return typedDigestBytes(digest.sum, digest.valid) }
func (digest PreauthCookieDigest) Bytes() ([]byte, error) {
	return typedDigestBytes(digest.sum, digest.valid)
}
func (digest PreauthProofDigest) Bytes() ([]byte, error) {
	return typedDigestBytes(digest.sum, digest.valid)
}
func (digest RecentAuthDigest) Bytes() ([]byte, error) {
	return typedDigestBytes(digest.sum, digest.valid)
}

func (digest SessionTokenDigest) Equal(other SessionTokenDigest) bool {
	return equalTypedDigest(digest.sum, digest.valid, other.sum, other.valid)
}

func (digest CSRFDigest) Equal(other CSRFDigest) bool {
	return equalTypedDigest(digest.sum, digest.valid, other.sum, other.valid)
}

func (digest PreauthCookieDigest) Equal(other PreauthCookieDigest) bool {
	return equalTypedDigest(digest.sum, digest.valid, other.sum, other.valid)
}

func (digest PreauthProofDigest) Equal(other PreauthProofDigest) bool {
	return equalTypedDigest(digest.sum, digest.valid, other.sum, other.valid)
}

func (digest RecentAuthDigest) Equal(other RecentAuthDigest) bool {
	return equalTypedDigest(digest.sum, digest.valid, other.sum, other.valid)
}

func digestOpaqueToken(key []byte, label string, token OpaqueToken) ([DigestBytes]byte, error) {
	if len(key) != DigestKeyBytes {
		return [DigestBytes]byte{}, ErrInvalidDigestKey
	}
	if !token.valid {
		return [DigestBytes]byte{}, ErrInvalidOpaqueToken
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte(label))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(token.value[:])
	var result [DigestBytes]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

func parseTypedDigest(value []byte) ([DigestBytes]byte, error) {
	if len(value) != DigestBytes {
		return [DigestBytes]byte{}, ErrInvalidTypedDigest
	}
	var result [DigestBytes]byte
	copy(result[:], value)
	return result, nil
}

func validateTypedDigest(valid bool) error {
	if !valid {
		return ErrInvalidTypedDigest
	}
	return nil
}

func typedDigestBytes(sum [DigestBytes]byte, valid bool) ([]byte, error) {
	if !valid {
		return nil, ErrInvalidTypedDigest
	}
	result := make([]byte, DigestBytes)
	copy(result, sum[:])
	return result, nil
}

func equalTypedDigest(left [DigestBytes]byte, leftValid bool, right [DigestBytes]byte, rightValid bool) bool {
	return leftValid && rightValid && subtle.ConstantTimeCompare(left[:], right[:]) == 1
}
