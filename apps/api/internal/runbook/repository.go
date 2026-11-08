package runbook

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const MaximumCorpora = 16
const MaximumVersion int64 = 9007199254740991

var (
	ErrDenied      = errors.New("runbook.denied")
	ErrConflict    = errors.New("runbook.conflict")
	ErrUnavailable = errors.New("runbook.unavailable")
)

// Revision is metadata only. A revision number or digest is never read authority.
type Revision struct {
	ID         string
	Version    int64
	Digest     [32]byte
	Audience   Audience
	Approved   bool
	Generation int64
	Documents  int
	Bytes      int
	Provenance Provenance
}

func (Revision) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook revision]")
}
func (Revision) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Change struct {
	Actor              auth.Principal
	Grant              auth.RecentGrantUse
	Corpus             Corpus
	ExpectedVersion    int64
	ExpectedGeneration int64
	Digest             [32]byte
	Revoke             bool
	Confirmed          bool
	ConfirmationDigest [32]byte
	Audit              domain.AuditEntry
}

func (Change) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook change]")
}
func (Change) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (change Change) Validate() error {
	if change.Actor.Validate() != nil || change.Actor.Role != auth.RoleAdministrator || change.Grant.Actor != change.Actor || change.Grant.Purpose != auth.RecentAuthPurposeRunbookManage || change.Grant.Digest.Validate() != nil || change.Grant.DigestKeyID == "" || !change.Confirmed || change.ConfirmationDigest == [32]byte{} || change.ExpectedVersion < 0 || change.ExpectedVersion > MaximumVersion || change.ExpectedGeneration < 0 || change.ExpectedGeneration > MaximumVersion || !identifier.MatchString(change.Corpus.ID) || change.Audit.Validate() != nil || change.Audit.Actor.Kind != domain.AuditActorUser || change.Audit.Actor.ID != string(change.Actor.UserID) || change.Audit.Target.Kind != "runbook.corpus" || change.Audit.Target.ID != change.Corpus.ID || change.Audit.Outcome != domain.AuditSuccess || !change.Audit.OccurredAt.Equal(change.Grant.ConsumedAt) {
		return ErrInvalid
	}
	if change.Revoke {
		if change.ExpectedVersion < 1 || change.Corpus.Version != change.ExpectedVersion || len(change.Corpus.Documents) != 0 || change.Corpus.Audience != "" || !change.Corpus.Provenance.Empty() || change.Digest == [32]byte{} || change.Audit.Action != "runbook.revoke" || change.Audit.ReasonCode != "runbook.revoked" {
			return ErrInvalid
		}
		return nil
	}
	digest, err := change.Corpus.Digest()
	if err != nil || change.ExpectedVersion >= MaximumVersion || change.Corpus.Version != change.ExpectedVersion+1 || subtle.ConstantTimeCompare(digest[:], change.Digest[:]) != 1 || change.Audit.Action != "runbook.approve" || change.Audit.ReasonCode != "runbook.approved" {
		return ErrInvalid
	}
	return nil
}

type Search struct {
	Actor          auth.Principal
	Query          string
	Locale         string
	Limit          int
	BeforeCorpus   string
	BeforeDocument string
	Generation     int64
	CatalogVersion int64
}

func (Search) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook query]")
}
func (Search) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (search Search) Validate() error {
	if search.Actor.Validate() != nil || Query(search.Query, search.Limit) != nil || search.Locale != "en" && search.Locale != "fa" || search.Generation < -1 || search.Generation > MaximumVersion || search.CatalogVersion > MaximumVersion || search.CatalogVersion < -1 || (search.Generation == -1) != (search.CatalogVersion == -1) || (search.BeforeCorpus == "") != (search.BeforeDocument == "") || search.BeforeCorpus != "" && (!identifier.MatchString(search.BeforeCorpus) || !identifier.MatchString(search.BeforeDocument) || search.Generation < 0) {
		return ErrInvalid
	}
	return nil
}

type Reference struct {
	CorpusID   string
	Version    int64
	Digest     [32]byte
	DocumentID string
	Locale     string
	Generation int64
}

func (reference Reference) Validate() error {
	if !identifier.MatchString(reference.CorpusID) || !identifier.MatchString(reference.DocumentID) || reference.Version < 1 || reference.Version > MaximumVersion || reference.Digest == [32]byte{} || reference.Locale != "en" && reference.Locale != "fa" || reference.Generation < 0 || reference.Generation > MaximumVersion {
		return ErrInvalid
	}
	return nil
}

type Hit struct {
	Reference Reference
	Title     string
	Snippet   string
}

func (Hit) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "[protected runbook match]") }
func (Hit) MarshalJSON() ([]byte, error)   { return []byte("{}"), nil }

type Page struct {
	Hits           []Hit
	HasMore        bool
	Generation     int64
	CatalogVersion int64
}

func (Page) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook matches]")
}
func (Page) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Repository interface {
	PreviewRunbook(context.Context, Preview) (PreviewResult, error)
	InspectRunbooks(context.Context, auth.Principal) (Catalog, error)
	ChangeRunbook(context.Context, Change) (Revision, error)
	SearchRunbooks(context.Context, Search) (Page, error)
	ReadRunbook(context.Context, auth.Principal, Reference) (Content, error)
}

type Preview struct {
	Actor              auth.Principal
	CorpusID           string
	ExpectedVersion    int64
	ExpectedGeneration int64
	Digest             [32]byte
	Revoke             bool
	ConfirmationDigest [32]byte
}

func (Preview) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook preview]")
}
func (Preview) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (preview Preview) Validate() error {
	if preview.Actor.Validate() != nil || preview.Actor.Role != auth.RoleAdministrator || !identifier.MatchString(preview.CorpusID) || preview.ExpectedVersion < 0 || preview.ExpectedVersion > MaximumVersion || preview.ExpectedGeneration < 0 || preview.ExpectedGeneration > MaximumVersion || preview.Digest == [32]byte{} || preview.ConfirmationDigest == [32]byte{} || preview.Revoke && preview.ExpectedVersion == 0 || !preview.Revoke && preview.ExpectedVersion >= MaximumVersion {
		return ErrInvalid
	}
	return nil
}

type Catalog struct {
	Generation int64
	Version    int64
	Revisions  []Revision
}

func (Catalog) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook catalog]")
}
func (Catalog) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type PreviewResult struct {
	Current   Revision
	ExpiresAt time.Time
}

func (PreviewResult) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook preview result]")
}
func (PreviewResult) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func ValidID(value string) bool { return identifier.MatchString(value) }
func (revision Revision) Validate() error {
	if !revision.Provenance.Empty() && revision.Provenance.Validate() != nil {
		return ErrInvalid
	}
	if !ValidID(revision.ID) || revision.Version < 1 || revision.Version > MaximumVersion || revision.Digest == [32]byte{} || revision.Audience != Administrators && revision.Audience != AllUsers || revision.Generation < 0 || revision.Generation > MaximumVersion || revision.Documents < 0 || revision.Documents > MaximumDocuments || revision.Bytes < 0 || revision.Bytes > MaximumCorpusBytes || revision.Approved && (revision.Documents == 0 || revision.Bytes == 0) || !revision.Approved && (revision.Documents != 0 || revision.Bytes != 0) {
		return ErrInvalid
	}
	return nil
}
func (hit Hit) Validate() error {
	if hit.Reference.Validate() != nil || (Document{ID: hit.Reference.DocumentID, Locale: hit.Reference.Locale, Title: hit.Title, Body: "x"}).Validate() != nil || len(hit.Snippet) < 1 || !text(hit.Snippet+"x", MaximumSnippetBytes+1, true) {
		return ErrInvalid
	}
	return nil
}

func Timestamp(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC && value.UnixMicro() >= 0 && value.Equal(value.Truncate(time.Microsecond))
}
