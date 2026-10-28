package httpapi

import (
	"errors"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantconversation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/go-chi/chi/v5"
)

type assistantHTTP struct {
	product   productHTTP
	runs      AssistantRunReader
	admitter  AssistantRunAdmitter
	bodyLimit int64
}

func (handler assistantHTTP) admit(w http.ResponseWriter, r *http.Request) {
	if !handler.product.auth.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	session, csrf, _, failure, ok := readAuthenticationMaterial(r, true, false)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	identity, err := handler.product.auth.service.AuthorizeUnsafe(r.Context(), session, csrf)
	if err != nil {
		handler.product.auth.writeAuthenticatedError(w, r, session, err)
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !assistantconversation.ValidNonce(keys[0]) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var input assistantconversation.Request
	if err := decodeJSONBody(r, handler.bodyLimit, &input); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	run, err := handler.admitter.Submit(r.Context(), actor, input, keys[0])
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	metadata, err := assistantMetadata(run, actor.UserID, run.ID)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	if metadata.Scope.SourceID != string(input.SourceID) || metadata.Scope.ResourceID != string(input.ResourceID) || metadata.Scope.WindowMinutes != input.WindowMinutes || (input.ConversationID != "" && metadata.ConversationID != input.ConversationID) {
		writeProblem(w, r, unavailableProblem)
		return
	}
	handler.product.auth.renewSessionCookie(w, r, session)
	writeJSON(w, http.StatusAccepted, assistantReceiptJSON{Run: metadata})
}

func assistantActor(identity auth.Authentication) (assistant.Actor, error) {
	if identity.Principal.Validate() != nil || identity.Session.UserID != identity.Principal.UserID || identity.Session.UserAuthRevision != identity.Principal.AuthRevision || identity.Session.ID != identity.Principal.SessionID {
		return assistant.Actor{}, assistant.ErrUnavailable
	}
	digest, err := identity.Session.TokenDigest.Bytes()
	if err != nil || len(digest) != 32 {
		clear(digest)
		return assistant.Actor{}, assistant.ErrUnavailable
	}
	defer clear(digest)
	actor := assistant.Actor{UserID: string(identity.Principal.UserID), DigestKeyID: identity.Session.TokenDigestKeyID, AuthRevision: identity.Principal.AuthRevision}
	copy(actor.SessionDigest[:], digest)
	if !actor.Valid() {
		return assistant.Actor{}, assistant.ErrUnavailable
	}
	return actor, nil
}

func assistantPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "run_id")
	if !requireNoQuery(w, r) {
		return "", false
	}
	if !assistantPublicRunID.MatchString(id) || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		writeProblem(w, r, invalidRequestProblem)
		return "", false
	}
	return id, true
}

func writeAssistantProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, assistant.ErrDenied):
		writeProblem(w, r, notFoundProblem)
	case errors.Is(err, assistant.ErrInvalid):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, assistant.ErrConflict):
		writeProblem(w, r, conflictProblem)
	case errors.Is(err, assistant.ErrQuota):
		writeProblem(w, r, rateLimitProblem)
	default:
		writeProblem(w, r, unavailableProblem)
	}
}

func (handler assistantHTTP) get(w http.ResponseWriter, r *http.Request) {
	identity, ok := handler.product.authenticateIdentity(w, r)
	if !ok {
		return
	}
	id, ok := assistantPath(w, r)
	if !ok {
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	snapshot, err := handler.runs.Get(r.Context(), actor, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	reply, err := assistantReply(snapshot, actor.UserID, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	writeAssistantReply(w, reply)
}

func (handler assistantHTTP) citation(w http.ResponseWriter, r *http.Request) {
	identity, ok := handler.product.authenticateIdentity(w, r)
	if !ok {
		return
	}
	id, ok := assistantPath(w, r)
	if !ok {
		return
	}
	citationID := chi.URLParam(r, "citation_id")
	if !assistantPublicCitationID.MatchString(citationID) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	snapshot, err := handler.runs.Get(r.Context(), actor, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	reply, err := assistantReply(snapshot, actor.UserID, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	if reply.ContentState != "ready" || len(reply.Citations) != 1 || reply.Citations[0].ID != citationID {
		writeProblem(w, r, notFoundProblem)
		return
	}
	writeJSON(w, http.StatusOK, reply.Citations[0])
}

func (handler assistantHTTP) cancel(w http.ResponseWriter, r *http.Request) {
	if !handler.product.auth.requireSameOrigin(w, r) {
		return
	}
	session, csrf, _, failure, ok := readAuthenticationMaterial(r, true, false)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	identity, err := handler.product.auth.service.AuthorizeUnsafe(r.Context(), session, csrf)
	if err != nil {
		handler.product.auth.writeAuthenticatedError(w, r, session, err)
		return
	}
	id, ok := assistantPath(w, r)
	if !ok {
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	run, err := handler.runs.Cancel(r.Context(), actor, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	metadata, err := assistantMetadata(run, actor.UserID, id)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	handler.product.auth.renewSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, assistantReceiptJSON{Run: metadata})
}
