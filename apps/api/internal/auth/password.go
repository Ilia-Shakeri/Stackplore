package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	MinimumPasswordBytes = 12
	MaximumPasswordBytes = 128

	defaultPasswordMemoryKiB   uint32 = 64 * 1024
	defaultPasswordTimeCost    uint32 = 3
	defaultPasswordParallelism uint8  = 1
	defaultPasswordSaltBytes   uint32 = 16
	defaultPasswordHashBytes   uint32 = 32
	defaultHashConcurrency            = 2

	minimumVerifierMemoryKiB   uint32 = 8 * 1024
	maximumVerifierMemoryKiB   uint32 = 256 * 1024
	maximumVerifierTimeCost    uint32 = 10
	maximumVerifierParallelism uint8  = 4
	minimumVerifierSaltBytes          = 16
	maximumVerifierSaltBytes          = 64
	minimumVerifierHashBytes          = 16
	maximumVerifierHashBytes          = 64
)

type passwordDeriver func([]byte, []byte, uint32, uint32, uint8, uint32) []byte

// PasswordHasher bounds memory-hard work and owns the approved write policy.
type PasswordHasher struct {
	policy PasswordPolicy
	random io.Reader
	slots  chan struct{}
	derive passwordDeriver
}

// DefaultPasswordPolicy returns the approved profile for new credentials.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		AlgorithmVersion:  argon2.Version,
		TimeCost:          defaultPasswordTimeCost,
		MemoryKiB:         defaultPasswordMemoryKiB,
		Parallelism:       defaultPasswordParallelism,
		SaltBytes:         defaultPasswordSaltBytes,
		HashBytes:         defaultPasswordHashBytes,
		MaximumConcurrent: defaultHashConcurrency,
	}
}

// NewPasswordHasher constructs a bounded password hasher.
func NewPasswordHasher(policy PasswordPolicy) (*PasswordHasher, error) {
	if !validWritePolicy(policy) {
		return nil, ErrInvalidPasswordPolicy
	}
	return &PasswordHasher{
		policy: policy,
		random: rand.Reader,
		slots:  make(chan struct{}, policy.MaximumConcurrent),
		derive: argon2.IDKey,
	}, nil
}

// Hash creates a new credential using the approved profile.
func (hasher *PasswordHasher) Hash(ctx context.Context, password []byte) (PasswordCredential, error) {
	if err := passwordContextError(ctx); err != nil {
		return PasswordCredential{}, err
	}
	if !validPassword(password) {
		return PasswordCredential{}, ErrInvalidPassword
	}
	if !hasher.valid() {
		return PasswordCredential{}, ErrPasswordHashUnavailable
	}

	salt := make([]byte, hasher.policy.SaltBytes)
	if _, err := io.ReadFull(hasher.random, salt); err != nil {
		clear(salt)
		return PasswordCredential{}, ErrPasswordHashUnavailable
	}
	hash, err := hasher.deriveBounded(ctx, password, salt, hasher.policy.TimeCost, hasher.policy.MemoryKiB, hasher.policy.Parallelism, hasher.policy.HashBytes)
	if err != nil {
		clear(salt)
		return PasswordCredential{}, err
	}
	return PasswordCredential{
		Algorithm:        PasswordAlgorithmArgon2id,
		AlgorithmVersion: hasher.policy.AlgorithmVersion,
		TimeCost:         hasher.policy.TimeCost,
		MemoryKiB:        hasher.policy.MemoryKiB,
		Parallelism:      hasher.policy.Parallelism,
		Salt:             salt,
		Hash:             hash,
	}, nil
}

// Verify checks one credential and reports whether a successful login should rehash it.
func (hasher *PasswordHasher) Verify(ctx context.Context, password []byte, credential PasswordCredential) (PasswordVerification, error) {
	if err := passwordContextError(ctx); err != nil {
		return PasswordVerification{}, err
	}
	if !validPassword(password) {
		return PasswordVerification{}, ErrInvalidPassword
	}
	if !hasher.valid() {
		return PasswordVerification{}, ErrPasswordHashUnavailable
	}
	if !validCredential(credential) {
		return PasswordVerification{}, ErrInvalidPasswordCredential
	}

	computed, err := hasher.deriveBounded(
		ctx,
		password,
		credential.Salt,
		credential.TimeCost,
		credential.MemoryKiB,
		credential.Parallelism,
		uint32(len(credential.Hash)),
	)
	if err != nil {
		return PasswordVerification{}, err
	}
	match := subtle.ConstantTimeCompare(computed, credential.Hash) == 1
	clear(computed)
	return PasswordVerification{
		Match:       match,
		NeedsRehash: match && !hasher.matchesWritePolicy(credential),
	}, nil
}

func (hasher *PasswordHasher) deriveBounded(
	ctx context.Context,
	password []byte,
	salt []byte,
	timeCost uint32,
	memoryKiB uint32,
	parallelism uint8,
	hashBytes uint32,
) ([]byte, error) {
	select {
	case hasher.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, errors.Join(ErrPasswordHashUnavailable, ctx.Err())
	}
	defer func() { <-hasher.slots }()

	hash := hasher.derive(password, salt, timeCost, memoryKiB, parallelism, hashBytes)
	if err := ctx.Err(); err != nil {
		clear(hash)
		return nil, errors.Join(ErrPasswordHashUnavailable, err)
	}
	return hash, nil
}

func (hasher *PasswordHasher) valid() bool {
	return hasher != nil &&
		validWritePolicy(hasher.policy) &&
		hasher.random != nil &&
		hasher.derive != nil &&
		cap(hasher.slots) == hasher.policy.MaximumConcurrent
}

func (hasher *PasswordHasher) matchesWritePolicy(credential PasswordCredential) bool {
	return credential.AlgorithmVersion == hasher.policy.AlgorithmVersion &&
		credential.TimeCost == hasher.policy.TimeCost &&
		credential.MemoryKiB == hasher.policy.MemoryKiB &&
		credential.Parallelism == hasher.policy.Parallelism &&
		len(credential.Salt) == int(hasher.policy.SaltBytes) &&
		len(credential.Hash) == int(hasher.policy.HashBytes)
}

func validWritePolicy(policy PasswordPolicy) bool {
	return policy.AlgorithmVersion == argon2.Version &&
		policy.TimeCost == defaultPasswordTimeCost &&
		policy.MemoryKiB == defaultPasswordMemoryKiB &&
		policy.Parallelism == defaultPasswordParallelism &&
		policy.SaltBytes == defaultPasswordSaltBytes &&
		policy.HashBytes == defaultPasswordHashBytes &&
		policy.MaximumConcurrent >= 1 &&
		policy.MaximumConcurrent <= defaultHashConcurrency
}

func validCredential(credential PasswordCredential) bool {
	return credential.Algorithm == PasswordAlgorithmArgon2id &&
		credential.AlgorithmVersion == argon2.Version &&
		credential.TimeCost >= 1 &&
		credential.TimeCost <= maximumVerifierTimeCost &&
		credential.MemoryKiB >= minimumVerifierMemoryKiB &&
		credential.MemoryKiB <= maximumVerifierMemoryKiB &&
		credential.Parallelism >= 1 &&
		credential.Parallelism <= maximumVerifierParallelism &&
		len(credential.Salt) >= minimumVerifierSaltBytes &&
		len(credential.Salt) <= maximumVerifierSaltBytes &&
		len(credential.Hash) >= minimumVerifierHashBytes &&
		len(credential.Hash) <= maximumVerifierHashBytes
}

func passwordContextError(ctx context.Context) error {
	if ctx == nil {
		return ErrPasswordHashUnavailable
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrPasswordHashUnavailable, err)
	}
	return nil
}

func validPassword(password []byte) bool {
	return len(password) >= MinimumPasswordBytes && len(password) <= MaximumPasswordBytes
}
