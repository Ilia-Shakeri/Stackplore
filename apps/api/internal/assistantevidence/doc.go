// Package assistantevidence implements the Smart Assistant evidence boundary.
//
// Trusted composition registers operator-owned sources once and shares a Reader.
// Registration pins the exact source configuration version. Configuration
// rotation invalidates old source memory and constructs a new Reader/provider
// binding; an old adapter cannot silently authorize a new source version.
// Read is one logical repository tool reservation. Its authority probes are
// bounded validation inside that reservation, not additional model-selected
// actions: at most four live inventory captures, three family probes, three
// selected-resource passes (32 resources each), six metadata checks, one support
// read and one family read. Log content adds one same-query policy check and one
// final metadata check. All work, including semaphore wait, shares a 25-second
// context and a two-operation provider limiter. Metadata-only checks have their
// own two-slot, three-second lane so two blocked reads cannot starve lease
// renewal. Neither lane owns background goroutines or unbounded queues. No
// probe follows a returned cursor.
//
// Source.Read may serve validated persisted observations. CaptureAuthority must
// receive the actual operator-bound provider, never a cached observation reader:
// inventory partitions prove resource scope; health, topology, current metrics,
// events and shared policy-applied log reads separately prove those families.
// A partial topology probe cannot authorize old derived edges. Missing ports,
// stale permission evidence and denied partitions fail closed. A support enum
// cannot grant authority. Demo timestamps describe fixed synthetic fixtures.
//
// AuthorizeAdmission checks only a zero-version candidate. Authorize checks
// current metadata only. Metadata polling uses the owning repository's read
// operation and never releases content or depends on provider reachability. Neither
// method replaces AuthorizeEvidence before dispatch, publication or replay of
// previously collected content. That method rechecks the sealed read family and
// exact log policy fingerprint. Citation resolution additionally requires the
// same owner/session and the successfully completed run from the same fence.
//
// Evidence is volatile, expires after 30 minutes from admission and must be
// erased by the owning service when its session, scope or run is invalidated.
// Explicit Bundle/Items access is for bounded transmission; ordinary formatting
// and serialization do not expose content. This package has no HTTP admission,
// generation, provider mutation, streaming, arbitrary query or file-path port.
package assistantevidence
