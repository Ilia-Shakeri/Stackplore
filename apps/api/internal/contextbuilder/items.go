package contextbuilder

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	MaximumItems      = 64
	MaximumItemBytes  = 8192
	MaximumItemsBytes = 64 << 10
)

// Item is a volatile, explicitly accessed projection, not a logging payload.
// Text is inert JSON data; it never grants authority or defines a link/action.
type Item struct {
	ID   string
	Text string
}

func (Item) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[protected evidence item]"))
}

func (Item) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type itemDocument struct {
	document
	ItemIndex int `json:"item_index"`
	ItemCount int `json:"item_count"`
}

// itemWire measures the explicit downstream id/text array, including escaping.
// Ordinary Item JSON remains protected; transmission must access its fields.
type itemWire struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Items returns all canonical citations or a safe typed error, never a prefix.
// Each item repeats its source/version/window/quality envelope and has one
// unchanged record. The index/count identifies this as one part of the bundle.
// The array cap includes id/text JSON framing, but a caller must also enforce
// its complete question/template byte and exact-token budgets before dispatch.
func (bundle Bundle) Items() ([]Item, error) {
	if bundle.document.Version != 1 || !canonicalItemID(bundle.document.SourceReference) || bundle.document.Provider.Validate() != nil ||
		bundle.document.ConfigurationVersion < 1 || bundle.document.PolicyVersion < 1 || bundle.document.RestoreGeneration < 0 ||
		!validTime(bundle.document.Range.Start) || !validTime(bundle.document.Range.End) || bundle.document.Range.End.Before(bundle.document.Range.Start) ||
		len(bundle.document.Records) != len(bundle.references) {
		return nil, ErrInvalid
	}
	if err := validateItemQuality(bundle.document.Quality); err != nil {
		return nil, err
	}
	count := len(bundle.document.Records)
	if count > MaximumItems {
		return nil, ErrBudget
	}
	items := make([]Item, 0, count)
	seen := make(map[string]bool, count)
	wireBytes := 2 // Array brackets, including for a valid empty bundle.
	for index, entry := range bundle.document.Records {
		reference := bundle.references[index]
		if !canonicalItemID(entry.CitationID) || seen[entry.CitationID] || reference.ID != entry.CitationID || reference.Kind != entry.Kind ||
			reference.Source.Validate() != nil || reference.Source.Provider != bundle.document.Provider ||
			reference.ConfigurationVersion != bundle.document.ConfigurationVersion || reference.PolicyVersion != bundle.document.PolicyVersion || reference.RestoreGeneration != bundle.document.RestoreGeneration ||
			!reference.Range.Start.Equal(bundle.document.Range.Start) || !reference.Range.End.Equal(bundle.document.Range.End) || !reference.ObservedAt.Equal(entry.Quality.ObservedAt) ||
			entry.Quality.Synthetic != bundle.document.Quality.Synthetic || entry.Log != nil && !utf8.ValidString(entry.Log.Message) {
			return nil, ErrInvalid
		}
		if err := validateItemQuality(entry.Quality); err != nil {
			return nil, err
		}
		seen[entry.CitationID] = true
		part := bundle.document
		part.Records = []record{entry}
		contents, err := json.Marshal(itemDocument{document: part, ItemIndex: index, ItemCount: count})
		if err != nil {
			return nil, ErrInvalid
		}
		if len(contents) > MaximumItemBytes {
			return nil, ErrBudget
		}
		item := Item{ID: entry.CitationID, Text: string(contents)}
		wire, _ := json.Marshal(itemWire{ID: item.ID, Text: item.Text})
		if index > 0 {
			wireBytes++
		}
		wireBytes += len(wire)
		if wireBytes > MaximumItemsBytes {
			return nil, ErrBudget
		}
		items = append(items, item)
	}
	return items, nil
}

func validateItemQuality(value quality) error {
	if value.Availability.Validate() != nil || value.Completeness.Validate() != nil || !validTime(value.ObservedAt) ||
		value.Completeness == domain.CompletenessPartial && value.Availability != domain.AvailabilityAvailable && value.Availability != domain.AvailabilityStale {
		return ErrInvalid
	}
	if value.Availability == domain.AvailabilityPermissionDenied {
		return ErrDenied
	}
	return nil
}

func canonicalItemID(value string) bool {
	if len(value) != 67 || !strings.HasPrefix(value, "ev_") || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[3:])
	return err == nil
}
