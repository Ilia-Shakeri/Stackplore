package contextbuilder

import (
	"encoding/hex"
	"fmt"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"io"
)

// ReviewedGuidance is bounded input from the owning read. Projection grants no rights.
type ReviewedGuidance struct {
	Reference runbook.Reference
	Content   runbook.Content
}

func (ReviewedGuidance) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected selected guidance]")
}
func (ReviewedGuidance) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type guidanceFact struct {
	CorpusID   string `json:"corpus_id"`
	Version    int64  `json:"version"`
	Digest     string `json:"digest"`
	DocumentID string `json:"document_id"`
	Locale     string `json:"locale"`
	Title      string `json:"title"`
	Excerpt    string `json:"excerpt"`
	Truncated  bool   `json:"truncated"`
	// This is guidance supplied by an operator, never a live infrastructure fact.
	Usage string `json:"usage"`
}

func (projection *projection) guidance() error {
	selected := projection.request.Guidance
	if selected == nil {
		return nil
	}
	ref, content := selected.Reference, selected.Content
	scope := projection.request.Scope
	if projection.request.Destination != Local || len(scope.Resources) != 1 || ref.Validate() != nil || ref.Generation != scope.RestoreGeneration || content.Validate() != nil || content.Provenance.Empty() || !content.Provenance.AssistanceReviewed || content.Document.ID != ref.DocumentID || content.Document.Locale != ref.Locale {
		return ErrDenied
	}
	excerpt, err := runbook.Excerpt(content.Document.Body, 256)
	if err != nil {
		return ErrInvalid
	}
	item := record{ResourceID: string(scope.Resources[0].ID), Quality: projection.quality(projection.request.Status), Guidance: &guidanceFact{CorpusID: ref.CorpusID, Version: ref.Version, Digest: hex.EncodeToString(ref.Digest[:]), DocumentID: ref.DocumentID, Locale: ref.Locale, Title: content.Document.Title, Excerpt: excerpt, Truncated: len(excerpt) < len(content.Document.Body), Usage: "reviewed_guidance_not_live_facts"}}
	item.Quality.ObservedAt = scope.ObservedAt
	if err := projection.add("document", ref.DocumentID, item); err != nil {
		return err
	}
	projection.bundle.references[len(projection.bundle.references)-1].Document = ref
	return nil
}
