package auth

const PasswordAlgorithmArgon2id = "argon2id"

// PasswordCredential is the versioned, non-plaintext password record.
// Salt and Hash must be treated as immutable by callers.
type PasswordCredential struct {
	Algorithm        string
	AlgorithmVersion int
	TimeCost         uint32
	MemoryKiB        uint32
	Parallelism      uint8
	Salt             []byte
	Hash             []byte
}

// Validate rejects records that could trigger unsafe verifier work.
func (credential PasswordCredential) Validate() error {
	if !validCredential(credential) {
		return ErrInvalidPasswordCredential
	}
	return nil
}

// PasswordVerification reports a constant-time credential comparison.
type PasswordVerification struct {
	Match       bool
	NeedsRehash bool
}

// PasswordPolicy defines the one approved profile for newly written records.
type PasswordPolicy struct {
	AlgorithmVersion  int
	TimeCost          uint32
	MemoryKiB         uint32
	Parallelism       uint8
	SaltBytes         uint32
	HashBytes         uint32
	MaximumConcurrent int
}

// Validate accepts only the approved write profile and a bounded worker count.
func (policy PasswordPolicy) Validate() error {
	if !validWritePolicy(policy) {
		return ErrInvalidPasswordPolicy
	}
	return nil
}
