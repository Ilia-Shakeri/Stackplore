package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"time"
)

var (
	ErrInvalidBootstrapGate = errors.New("bootstrap gate configuration is invalid")
	ErrBootstrapDenied      = errors.New("bootstrap proof is not accepted")
	ErrBootstrapUnavailable = errors.New("bootstrap gate is unavailable")
)

// BootstrapGate serializes the one transition allowed by an operator proof.
type BootstrapGate struct {
	lock        chan struct{}
	proofDigest [sha256.Size]byte
	expiresAt   time.Time
	used        bool
	now         func() time.Time
}

// NewBootstrapGate keeps only a digest of one canonical 256-bit operator proof.
func NewBootstrapGate(encodedProof string, expiresAt time.Time) (*BootstrapGate, error) {
	return newBootstrapGate(encodedProof, expiresAt, time.Now)
}

func newBootstrapGate(encodedProof string, expiresAt time.Time, now func() time.Time) (*BootstrapGate, error) {
	if now == nil || expiresAt.IsZero() {
		return nil, ErrInvalidBootstrapGate
	}
	if _, err := ParseOpaqueToken(encodedProof); err != nil {
		return nil, ErrInvalidBootstrapGate
	}
	current := now()
	if current.IsZero() || !expiresAt.After(current) {
		return nil, ErrInvalidBootstrapGate
	}
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &BootstrapGate{
		lock:        lock,
		proofDigest: sha256.Sum256([]byte(encodedProof)),
		expiresAt:   expiresAt,
		now:         now,
	}, nil
}

// Use holds a matching unexpired proof across apply and consumes it only after apply succeeds.
func (gate *BootstrapGate) Use(ctx context.Context, candidate string, apply func(context.Context) error) error {
	if ctx == nil || gate == nil || gate.lock == nil || gate.now == nil || apply == nil {
		return ErrBootstrapUnavailable
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrBootstrapUnavailable, err)
	}

	select {
	case <-gate.lock:
	case <-ctx.Done():
		return errors.Join(ErrBootstrapUnavailable, ctx.Err())
	}
	defer func() { gate.lock <- struct{}{} }()

	candidateDigest, candidateValid := fixedBootstrapDigest(candidate)
	proofMatches := subtle.ConstantTimeCompare(candidateDigest[:], gate.proofDigest[:]) == 1
	current := gate.now()
	if !candidateValid || !proofMatches || gate.used || current.IsZero() || !current.Before(gate.expiresAt) {
		return ErrBootstrapDenied
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrBootstrapUnavailable, err)
	}
	if err := apply(ctx); err != nil {
		return err
	}
	gate.used = true
	clear(gate.proofDigest[:])
	return nil
}

func fixedBootstrapDigest(candidate string) ([sha256.Size]byte, bool) {
	var fixed [OpaqueTokenEncodedSize]byte
	valid := len(candidate) == len(fixed)
	if valid {
		copy(fixed[:], candidate)
		_, err := ParseOpaqueToken(candidate)
		valid = err == nil
	}
	digest := sha256.Sum256(fixed[:])
	clear(fixed[:])
	return digest, valid
}
