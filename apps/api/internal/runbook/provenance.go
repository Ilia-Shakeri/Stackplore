package runbook

import (
	"fmt"
	"io"
)

// Provenance records an operator's review, not a legal or factual certification.
// Source and licence are inert attribution text; neither is fetched or executed.
type Provenance struct {
	Kind        string
	Source      string
	License     string
	UseReviewed bool
	AssistanceReviewed bool
}

func (Provenance) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected document provenance]")
}
func (Provenance) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (p Provenance) Empty() bool                { return p == (Provenance{}) }
func (p Provenance) Validate() error {
	if (p.Kind != "operator_authored" && p.Kind != "licensed_copy" && p.Kind != "public_domain") || !text(p.Source, 512, false) || !text(p.License, 128, false) || !p.UseReviewed {
		return ErrInvalid
	}
	return nil
}

// Content keeps stored source attribution separate from importable document bytes.
// Empty provenance denotes a pre-migration revision, never an inferred permission.
type Content struct {
	Document   Document
	Provenance Provenance
}

func (Content) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected reviewed content]")
}
func (Content) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (c Content) Validate() error {
	if c.Document.Validate() != nil || !c.Provenance.Empty() && c.Provenance.Validate() != nil {
		return ErrInvalid
	}
	return nil
}
