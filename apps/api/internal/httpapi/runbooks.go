package httpapi

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbookservice"
)

// Runbooks exposes reviewed prose through the current account boundary.
type Runbooks interface {
	Catalog(context.Context, auth.Principal) (runbook.Catalog, error)
	Preview(context.Context, auth.Principal, runbookservice.Material) (runbookservice.Draft, error)
	Commit(context.Context, auth.Principal, runbookservice.Material, string, bool, auth.RecentGrantUse, domain.RequestID) (runbook.Revision, error)
	Search(context.Context, auth.Principal, string, string, int, string) (runbookservice.Results, error)
	Read(context.Context, auth.Principal, runbook.Reference) (runbook.Content, error)
}

type runbookHTTP struct {
	product   productHTTP
	service   Runbooks
	bodyLimit int64
}
type runbookDocumentJSON struct {
	ID     string `json:"id"`
	Locale string `json:"locale"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}
type runbookProvenanceJSON struct {
 Kind string `json:"kind"`
 Source string `json:"source"`
 License string `json:"license"`
 UseReviewed bool `json:"use_reviewed"`
}
func runbookProvenanceWire(value runbook.Provenance) *runbookProvenanceJSON {
 if value.Empty() { return nil }
 return &runbookProvenanceJSON{value.Kind,value.Source,value.License,value.UseReviewed}
}
type runbookContentJSON struct {
 Document runbookDocumentJSON `json:"document"`
 Provenance *runbookProvenanceJSON `json:"provenance"`
}
type runbookCorpusJSON struct {
	ID        string                `json:"id"`
	Version   int64                 `json:"version"`
	Audience  runbook.Audience      `json:"audience"`
	Documents []runbookDocumentJSON `json:"documents"`
	Provenance *runbookProvenanceJSON `json:"provenance"`
}
type runbookMaterialJSON struct {
	Action          string            `json:"action"`
	ExpectedVersion *int64            `json:"expected_version"`
	Generation      *int64            `json:"generation"`
	Digest          string            `json:"digest"`
	Corpus          runbookCorpusJSON `json:"corpus"`
}

func (runbookMaterialJSON) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected document material]")
}

func (runbookMaterialJSON) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (wire runbookMaterialJSON) material() (runbookservice.Material, error) {
	if wire.ExpectedVersion == nil || wire.Generation == nil || wire.Corpus.Documents == nil {
		return runbookservice.Material{}, runbook.ErrInvalid
	}
	corpus := runbook.Corpus{ID: wire.Corpus.ID, Version: wire.Corpus.Version, Audience: wire.Corpus.Audience, Documents: make([]runbook.Document, len(wire.Corpus.Documents))}
	if p := wire.Corpus.Provenance; p != nil { corpus.Provenance = runbook.Provenance{Kind:p.Kind,Source:p.Source,License:p.License,UseReviewed:p.UseReviewed} }
	for i, document := range wire.Corpus.Documents {
		corpus.Documents[i] = runbook.Document{ID: document.ID, Locale: document.Locale, Title: document.Title, Body: document.Body}
	}
	material := runbookservice.Material{Corpus: corpus, ExpectedVersion: *wire.ExpectedVersion, Generation: *wire.Generation}
	switch wire.Action {
	case "approve":
		if wire.Digest != "" {
			return runbookservice.Material{}, runbook.ErrInvalid
		}
		digest, err := corpus.Digest()
		if err != nil {
			return runbookservice.Material{}, runbook.ErrInvalid
		}
		material.Digest = digest
	case "revoke":
		digest, err := runbookHexDigest(wire.Digest)
		if err != nil {
			return runbookservice.Material{}, err
		}
		material.Digest = digest
		material.Revoke = true
	default:
		return runbookservice.Material{}, runbook.ErrInvalid
	}
	return material, nil
}
func runbookHexDigest(value string) ([32]byte, error) {
	var digest [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != value {
		return digest, runbook.ErrInvalid
	}
	copy(digest[:], decoded)
	if digest == [32]byte{} {
		return digest, runbook.ErrInvalid
	}
	return digest, nil
}

type runbookRevisionJSON struct {
	ID         string           `json:"id"`
	Version    int64            `json:"version"`
	Digest     string           `json:"digest"`
	Audience   runbook.Audience `json:"audience"`
	Approved   bool             `json:"approved"`
	Generation int64            `json:"generation"`
	Documents  int              `json:"documents"`
	Bytes      int              `json:"bytes"`
	Provenance *runbookProvenanceJSON `json:"provenance"`
}

func runbookRevisionWire(value runbook.Revision) runbookRevisionJSON {
	return runbookRevisionJSON{value.ID, value.Version, hex.EncodeToString(value.Digest[:]), value.Audience, value.Approved, value.Generation, value.Documents, value.Bytes,runbookProvenanceWire(value.Provenance)}
}

type runbookReferenceJSON struct {
	CorpusID   string `json:"corpus_id"`
	Version    int64  `json:"version"`
	Digest     string `json:"digest"`
	DocumentID string `json:"document_id"`
	Locale     string `json:"locale"`
	Generation int64  `json:"generation"`
}

func runbookReferenceWire(value runbook.Reference) runbookReferenceJSON {
	return runbookReferenceJSON{value.CorpusID, value.Version, hex.EncodeToString(value.Digest[:]), value.DocumentID, value.Locale, value.Generation}
}
func (wire runbookReferenceJSON) reference() (runbook.Reference, error) {
	digest, err := runbookHexDigest(wire.Digest)
	if err != nil {
		return runbook.Reference{}, err
	}
	value := runbook.Reference{CorpusID: wire.CorpusID, Version: wire.Version, Digest: digest, DocumentID: wire.DocumentID, Locale: wire.Locale, Generation: wire.Generation}
	return value, value.Validate()
}

func (handler runbookHTTP) authorize(w http.ResponseWriter, r *http.Request, admin, recent bool) (auth.Authentication, auth.RecentGrantUse, bool) {
	boundary := handler.product.auth
	if !boundary.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	token, csrf, proof, failure, ok := readAuthenticationMaterial(r, true, recent)
	if !ok {
		writeProblem(w, r, failure)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	identity, err := boundary.service.AuthorizeUnsafe(r.Context(), token, csrf)
	if err != nil {
		boundary.writeAuthenticatedError(w, r, token, err)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	if admin && identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	if handler.service == nil {
		writeProblem(w, r, unavailableProblem)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	if !recent {
		return identity, auth.RecentGrantUse{}, true
	}
	preparer, ok := boundary.service.(applicationMutationPreparer)
	if !ok {
		writeProblem(w, r, unavailableProblem)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	identity, grant, err := preparer.PrepareRecentMutation(r.Context(), auth.RecentAuthorizationInput{SessionToken: token, CSRFProof: csrf, RecentProof: proof, Purpose: auth.RecentAuthPurposeRunbookManage, RequestID: requestIDForAuth(r)})
	if err != nil {
		boundary.writeAuthenticatedError(w, r, token, err)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	return identity, grant, true
}
func writeRunbookError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, runbook.ErrInvalid):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, runbook.ErrDenied):
		writeProblem(w, r, forbiddenProblem)
	case errors.Is(err, runbook.ErrConflict):
		writeProblem(w, r, problemKind{status: 409, code: "runbook.version_conflict", title: "Document catalog changed"})
	default:
		writeProblem(w, r, unavailableProblem)
	}
}
func (handler runbookHTTP) catalog(w http.ResponseWriter, r *http.Request) {
	if !requireNoQuery(w, r) {
		return
	}
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	identity, ok := handler.product.authenticateIdentity(w, r)
	if !ok {
		return
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	if handler.service == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	result, err := handler.service.Catalog(r.Context(), identity.Principal)
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	revisions := make([]runbookRevisionJSON, len(result.Revisions))
	for i, revision := range result.Revisions {
		revisions[i] = runbookRevisionWire(revision)
	}
	writeJSON(w, http.StatusOK, struct {
		Generation          int64                 `json:"generation"`
		Version             int64                 `json:"version"`
		Revisions           []runbookRevisionJSON `json:"revisions"`
		MaximumRequestBytes int64                 `json:"maximum_request_bytes"`
	}{result.Generation, result.Version, revisions, handler.bodyLimit})
}
func (handler runbookHTTP) preview(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := handler.authorize(w, r, true, false)
	if !ok {
		return
	}
	var request runbookMaterialJSON
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	material, err := request.material()
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	result, err := handler.service.Preview(r.Context(), identity.Principal, material)
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Digest            string `json:"digest"`
		ConfirmationToken string `json:"confirmation_token"`
		ExpiresAt         string `json:"expires_at"`
	}{hex.EncodeToString(material.Digest[:]), result.Token, authTimestamp(result.ExpiresAt)})
}
func (handler runbookHTTP) commit(w http.ResponseWriter, r *http.Request) {
	identity, grant, ok := handler.authorize(w, r, true, true)
	if !ok {
		return
	}
	var request struct {
		Material          runbookMaterialJSON `json:"material"`
		ConfirmationToken secretInput         `json:"confirmation_token"`
		Confirmed         *bool               `json:"confirmed"`
	}
	defer func() { clearSecret(request.ConfirmationToken) }()
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	if request.Confirmed == nil || !*request.Confirmed {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	material, err := request.Material.material()
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	result, err := handler.service.Commit(r.Context(), identity.Principal, material, string(request.ConfirmationToken), true, grant, requestIDForAuth(r))
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, runbookRevisionWire(result))
}
func (handler runbookHTTP) search(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := handler.authorize(w, r, false, false)
	if !ok {
		return
	}
	var request struct {
		Query  string `json:"query"`
		Locale string `json:"locale"`
		Limit  int    `json:"limit"`
		Cursor string `json:"cursor"`
	}
	if err := decodeJSONBody(r, min(handler.bodyLimit, maximumAuthBodyBytes), &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	result, err := handler.service.Search(r.Context(), identity.Principal, request.Query, request.Locale, request.Limit, request.Cursor)
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	type hit struct {
		Reference runbookReferenceJSON `json:"reference"`
		Title     string               `json:"title"`
		Snippet   string               `json:"snippet"`
	}
	hits := make([]hit, len(result.Hits))
	for i, item := range result.Hits {
		hits[i] = hit{runbookReferenceWire(item.Reference), item.Title, item.Snippet}
	}
	writeJSON(w, http.StatusOK, struct {
		Hits       []hit  `json:"hits"`
		NextCursor string `json:"next_cursor"`
	}{hits, result.Cursor})
}
func (handler runbookHTTP) document(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := handler.authorize(w, r, false, false)
	if !ok {
		return
	}
	var request runbookReferenceJSON
	if err := decodeJSONBody(r, min(handler.bodyLimit, maximumAuthBodyBytes), &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	reference, err := request.reference()
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	result, err := handler.service.Read(r.Context(), identity.Principal, reference)
	if err != nil {
		writeRunbookError(w, r, err)
		return
	}
	doc := result.Document
	writeJSON(w, http.StatusOK, runbookContentJSON{runbookDocumentJSON{doc.ID, doc.Locale, doc.Title, doc.Body},runbookProvenanceWire(result.Provenance)})
}
