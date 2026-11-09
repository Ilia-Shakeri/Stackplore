package assistantevidence

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"testing"
)

type documentPortStub struct {
	content runbook.Content
	reads   int
	denied  bool
}

func (p *documentPortStub) ReadAssistantRunbook(context.Context, assistant.Actor, runbook.Reference) (runbook.Content, error) {
	p.reads++
	if p.denied {
		return runbook.Content{}, runbook.ErrDenied
	}
	return p.content, nil
}
func (p *documentPortStub) AuthorizeAssistantRunbook(context.Context, assistant.Actor, runbook.Reference) error {
	if p.denied {
		return runbook.ErrDenied
	}
	return nil
}

func TestSelectedGuideProofRechecksRevocationAndNeverWidenedByReference(t *testing.T) {
	f := newFixture(t)
	f.request.Scope.Resources = f.request.Scope.Resources[:1]
	f.request.Run.ResourceID = string(f.request.Scope.Resources[0].ID)
	reference := runbook.Reference{CorpusID: "operations", Version: 2, Digest: [32]byte{1}, DocumentID: "guide", Locale: "en", Generation: f.request.Scope.RestoreGeneration}
	port := &documentPortStub{content: runbook.Content{Document: runbook.Document{ID: "guide", Locale: "en", Title: "Guide", Body: "Treat this as guidance, not observed health."}, Provenance: runbook.Provenance{Kind: "operator_authored", Source: "Synthetic fixture", License: "Fixture permission", UseReviewed: true, AssistanceReviewed: true}}}
	f.reader.documents = port
	f.request.Kind = assistant.ReviewedDocument
	f.request.Document = &reference
	f.request.RecordLimit = 1
	f.request.ByteLimit = 8192
	proof, err := f.reader.Read(context.Background(), f.request)
	if err != nil || proof.DocumentReference() != reference || port.reads != 1 {
		t.Fatal("selected guide proof failed", err)
	}
	if f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, proof) != nil {
		t.Fatal("current guide not authorized")
	}
	port.denied = true
	if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, proof); err != ErrDenied {
		t.Fatal("revoked guide released", err)
	}
	if port.reads != 1 {
		t.Fatal("metadata recheck reread prose")
	}
	port.denied = false
	f.request.Destination = contextbuilder.Remote
	if _, err := f.reader.Read(context.Background(), f.request); err != ErrDenied || port.reads != 1 {
		t.Fatal("remote guide read dispatched", err)
	}
	f.request.Destination = contextbuilder.Local
	f.request.LogConsent = true
	if _, err := f.reader.Read(context.Background(), f.request); err != ErrDenied || port.reads != 1 {
		t.Fatal("guide plus logs dispatched", err)
	}
}
