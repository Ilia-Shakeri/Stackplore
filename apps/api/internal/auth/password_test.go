package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
)

func TestDefaultPasswordPolicy(t *testing.T) {
	policy := DefaultPasswordPolicy()
	if policy.AlgorithmVersion != 19 ||
		policy.MemoryKiB != 64*1024 ||
		policy.TimeCost != 3 ||
		policy.Parallelism != 1 ||
		policy.SaltBytes != 16 ||
		policy.HashBytes != 32 ||
		policy.MaximumConcurrent != 2 {
		t.Fatalf("default policy = %+v", policy)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("default policy validation error = %v", err)
	}
}

func TestPasswordPolicyRejectsUnapprovedProfiles(t *testing.T) {
	base := DefaultPasswordPolicy()
	tests := map[string]func(*PasswordPolicy){
		"version":     func(policy *PasswordPolicy) { policy.AlgorithmVersion-- },
		"time":        func(policy *PasswordPolicy) { policy.TimeCost-- },
		"memory":      func(policy *PasswordPolicy) { policy.MemoryKiB-- },
		"parallelism": func(policy *PasswordPolicy) { policy.Parallelism++ },
		"salt":        func(policy *PasswordPolicy) { policy.SaltBytes++ },
		"hash":        func(policy *PasswordPolicy) { policy.HashBytes++ },
		"no workers":  func(policy *PasswordPolicy) { policy.MaximumConcurrent = 0 },
		"many workers": func(policy *PasswordPolicy) {
			policy.MaximumConcurrent = 3
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			policy := base
			mutate(&policy)
			if err := policy.Validate(); !errors.Is(err, ErrInvalidPasswordPolicy) {
				t.Fatalf("Validate() error = %v", err)
			}
			if _, err := NewPasswordHasher(policy); !errors.Is(err, ErrInvalidPasswordPolicy) {
				t.Fatalf("NewPasswordHasher() error = %v", err)
			}
		})
	}
}

func TestPasswordHasherHashesAndVerifiesApprovedProfile(t *testing.T) {
	hasher := mustPasswordHasher(t)
	salt := bytes.Repeat([]byte{0x5a}, 16)
	hasher.random = bytes.NewReader(salt)
	password := []byte("correct horse battery staple")

	credential, err := hasher.Hash(context.Background(), password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	wantHash := argon2.IDKey(password, salt, 3, 64*1024, 1, 32)
	if credential.Algorithm != PasswordAlgorithmArgon2id ||
		credential.AlgorithmVersion != argon2.Version ||
		!bytes.Equal(credential.Salt, salt) ||
		!bytes.Equal(credential.Hash, wantHash) {
		t.Fatal("Hash() did not use the approved profile")
	}
	if err := credential.Validate(); err != nil {
		t.Fatalf("credential validation error = %v", err)
	}

	verified, err := hasher.Verify(context.Background(), password, credential)
	if err != nil || !verified.Match || verified.NeedsRehash {
		t.Fatalf("Verify(correct) = %+v, %v", verified, err)
	}
	verified, err = hasher.Verify(context.Background(), []byte("wrong password value"), credential)
	if err != nil || verified.Match || verified.NeedsRehash {
		t.Fatalf("Verify(wrong) = %+v, %v", verified, err)
	}
}

func TestPasswordHasherEnforcesByteBoundsWithoutChangingInput(t *testing.T) {
	hasher := mustPasswordHasher(t)
	hasher.random = repeatingReader{value: 1}
	hasher.derive = fixedDeriver(7)

	for _, size := range []int{MinimumPasswordBytes - 1, MaximumPasswordBytes + 1} {
		password := bytes.Repeat([]byte{'p'}, size)
		if _, err := hasher.Hash(context.Background(), password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("Hash(%d bytes) error = %v", size, err)
		}
	}
	password := append([]byte{0, 1}, bytes.Repeat([]byte{'p'}, MinimumPasswordBytes-2)...)
	want := append([]byte(nil), password...)
	if _, err := hasher.Hash(context.Background(), password); err != nil {
		t.Fatalf("Hash(binary password) error = %v", err)
	}
	if !bytes.Equal(password, want) {
		t.Fatal("Hash() changed password bytes")
	}
}

func TestPasswordCredentialVerifierCeilings(t *testing.T) {
	valid := PasswordCredential{
		Algorithm:        PasswordAlgorithmArgon2id,
		AlgorithmVersion: argon2.Version,
		TimeCost:         1,
		MemoryKiB:        minimumVerifierMemoryKiB,
		Parallelism:      1,
		Salt:             make([]byte, minimumVerifierSaltBytes),
		Hash:             make([]byte, minimumVerifierHashBytes),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid credential error = %v", err)
	}
	tests := map[string]func(*PasswordCredential){
		"algorithm":   func(value *PasswordCredential) { value.Algorithm = "other" },
		"version":     func(value *PasswordCredential) { value.AlgorithmVersion = 18 },
		"zero time":   func(value *PasswordCredential) { value.TimeCost = 0 },
		"high time":   func(value *PasswordCredential) { value.TimeCost = maximumVerifierTimeCost + 1 },
		"low memory":  func(value *PasswordCredential) { value.MemoryKiB = minimumVerifierMemoryKiB - 1 },
		"high memory": func(value *PasswordCredential) { value.MemoryKiB = maximumVerifierMemoryKiB + 1 },
		"zero lanes":  func(value *PasswordCredential) { value.Parallelism = 0 },
		"many lanes":  func(value *PasswordCredential) { value.Parallelism = maximumVerifierParallelism + 1 },
		"short salt":  func(value *PasswordCredential) { value.Salt = make([]byte, minimumVerifierSaltBytes-1) },
		"long salt":   func(value *PasswordCredential) { value.Salt = make([]byte, maximumVerifierSaltBytes+1) },
		"short hash":  func(value *PasswordCredential) { value.Hash = make([]byte, minimumVerifierHashBytes-1) },
		"long hash":   func(value *PasswordCredential) { value.Hash = make([]byte, maximumVerifierHashBytes+1) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			credential := valid
			mutate(&credential)
			if err := credential.Validate(); !errors.Is(err, ErrInvalidPasswordCredential) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	hasher := mustPasswordHasher(t)
	hasher.derive = func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
		panic("unsafe credential reached password worker")
	}
	unsafe := valid
	unsafe.MemoryKiB = maximumVerifierMemoryKiB + 1
	if _, err := hasher.Verify(context.Background(), []byte("valid password bytes"), unsafe); !errors.Is(err, ErrInvalidPasswordCredential) {
		t.Fatalf("Verify(unsafe) error = %v", err)
	}
}

func TestPasswordHasherReportsRehashOnlyAfterMatch(t *testing.T) {
	hasher := mustPasswordHasher(t)
	hasher.derive = fixedDeriver(9)
	credential := PasswordCredential{
		Algorithm:        PasswordAlgorithmArgon2id,
		AlgorithmVersion: argon2.Version,
		TimeCost:         2,
		MemoryKiB:        32 * 1024,
		Parallelism:      1,
		Salt:             bytes.Repeat([]byte{1}, 16),
		Hash:             bytes.Repeat([]byte{9}, 32),
	}
	result, err := hasher.Verify(context.Background(), []byte("valid password bytes"), credential)
	if err != nil || !result.Match || !result.NeedsRehash {
		t.Fatalf("Verify() = %+v, %v", result, err)
	}
	credential.Hash[0] = 8
	result, err = hasher.Verify(context.Background(), []byte("valid password bytes"), credential)
	if err != nil || result.Match || result.NeedsRehash {
		t.Fatalf("Verify(mismatch) = %+v, %v", result, err)
	}
}

func TestPasswordHasherMapsRandomFailure(t *testing.T) {
	hasher := mustPasswordHasher(t)
	hasher.random = errorReader{}
	if _, err := hasher.Hash(context.Background(), []byte("valid password bytes")); !errors.Is(err, ErrPasswordHashUnavailable) {
		t.Fatalf("Hash() error = %v", err)
	}
}

func TestPasswordHasherCapsConcurrentWorkAtTwo(t *testing.T) {
	hasher := mustPasswordHasher(t)
	hasher.random = repeatingReader{value: 3}
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	var current atomic.Int32
	var peak atomic.Int32
	hasher.derive = func(_ []byte, _ []byte, _ uint32, _ uint32, _ uint8, length uint32) []byte {
		active := current.Add(1)
		for {
			seen := peak.Load()
			if active <= seen || peak.CompareAndSwap(seen, active) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		current.Add(-1)
		return make([]byte, length)
	}

	password := []byte("valid password bytes")
	var group sync.WaitGroup
	group.Add(2)
	for range 2 {
		go func() {
			defer group.Done()
			_, _ = hasher.Hash(context.Background(), password)
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("two hash workers did not start")
		}
	}

	thirdContext, cancelThird := context.WithCancel(context.Background())
	thirdDone := make(chan error, 1)
	go func() {
		_, err := hasher.Hash(thirdContext, password)
		thirdDone <- err
	}()
	select {
	case <-entered:
		t.Fatal("third hash worker started")
	case <-time.After(25 * time.Millisecond):
	}
	cancelThird()
	if err := <-thirdDone; !errors.Is(err, context.Canceled) || !errors.Is(err, ErrPasswordHashUnavailable) {
		t.Fatalf("third Hash() error = %v", err)
	}
	close(release)
	group.Wait()
	if got := peak.Load(); got != 2 {
		t.Fatalf("peak workers = %d", got)
	}
}

func TestPasswordHasherRejectsCanceledAndInvalidState(t *testing.T) {
	hasher := mustPasswordHasher(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hasher.Hash(canceled, []byte("valid password bytes")); !errors.Is(err, context.Canceled) || !errors.Is(err, ErrPasswordHashUnavailable) {
		t.Fatalf("Hash(canceled) error = %v", err)
	}
	if _, err := (*PasswordHasher)(nil).Hash(context.Background(), []byte("valid password bytes")); !errors.Is(err, ErrPasswordHashUnavailable) {
		t.Fatalf("nil Hash() error = %v", err)
	}
}

func BenchmarkPasswordHasher(b *testing.B) {
	hasher, err := NewPasswordHasher(DefaultPasswordPolicy())
	if err != nil {
		b.Fatalf("NewPasswordHasher() error = %v", err)
	}
	hasher.random = repeatingReader{value: 5}
	password := []byte("benchmark password value")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := hasher.Hash(context.Background(), password); err != nil {
			b.Fatalf("Hash() error = %v", err)
		}
	}
}

func mustPasswordHasher(t *testing.T) *PasswordHasher {
	t.Helper()
	hasher, err := NewPasswordHasher(DefaultPasswordPolicy())
	if err != nil {
		t.Fatalf("NewPasswordHasher() error = %v", err)
	}
	return hasher
}

func fixedDeriver(value byte) passwordDeriver {
	return func(_ []byte, _ []byte, _ uint32, _ uint32, _ uint8, length uint32) []byte {
		return bytes.Repeat([]byte{value}, int(length))
	}
}

type repeatingReader struct {
	value byte
}

func (reader repeatingReader) Read(target []byte) (int, error) {
	for index := range target {
		target[index] = reader.value
	}
	return len(target), nil
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
