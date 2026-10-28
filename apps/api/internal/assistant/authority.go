package assistant

import (
	"context"
	"time"
)

// EvidenceAuthority verifies current metadata authority before evidence is read
// or released. Providers must separately verify their current read partition.
type EvidenceAuthority interface {
	// Admission is a separate purpose for trusted, zero-version queued candidates.
	// It makes no claim that a persisted run or output already exists.
	AuthorizeAssistantAdmission(context.Context, Actor, Run, time.Time) error
	AuthorizeAssistantEvidence(context.Context, Actor, Run, time.Time) error
}
