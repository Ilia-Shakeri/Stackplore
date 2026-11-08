// Package runbook validates inert, explicitly reviewed document revisions.
package runbook

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"regexp"
	"sort"
	"unicode"
	"unicode/utf8"
)

const (
	MaximumDocuments     = 32
	MaximumDocumentBytes = 16 * 1024
	MaximumCorpusBytes   = 512 * 1024
	MaximumTitleBytes    = 256
	MaximumQueryBytes    = 256
	MaximumResults       = 10
	MaximumSnippetBytes  = 512
)

var ErrInvalid = errors.New("runbook.invalid")

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type Audience string

const (
	Administrators Audience = "administrators"
	AllUsers       Audience = "all_users"
)

// Document contains operator-supplied inert prose, never a command or tool plan.
type Document struct {
	ID     string
	Locale string
	Title  string
	Body   string
}

func (Document) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook document]")
}
func (Document) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// Corpus is a candidate value. Validation and hashing grant no approval or read rights.
type Corpus struct {
	ID        string
	Version   int64
	Audience  Audience
	Documents []Document
}

func (Corpus) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected runbook corpus]")
}
func (Corpus) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func text(value string, maximum int, multiline bool) bool {
	if len(value) == 0 || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	visible := false
	for _, char := range value {
		if unicode.IsControl(char) && (!multiline || char != '\n' && char != '\t') || unicode.Is(unicode.Cf, char) && char != '\u200c' && char != '\u200d' {
			return false
		}
		visible = visible || !unicode.IsSpace(char)
	}
	return visible
}

func (document Document) Validate() error {
	if !identifier.MatchString(document.ID) || document.Locale != "en" && document.Locale != "fa" || !text(document.Title, MaximumTitleBytes, false) || !text(document.Body, MaximumDocumentBytes, true) {
		return ErrInvalid
	}
	return nil
}

func (corpus Corpus) Validate() error {
	if !identifier.MatchString(corpus.ID) || corpus.Version < 1 || corpus.Version > MaximumVersion || corpus.Audience != Administrators && corpus.Audience != AllUsers || len(corpus.Documents) < 1 || len(corpus.Documents) > MaximumDocuments {
		return ErrInvalid
	}
	seen := make(map[string]bool, len(corpus.Documents))
	bytes := len(corpus.ID) + len(corpus.Audience)
	for _, document := range corpus.Documents {
		key := document.ID + ":" + document.Locale
		if document.Validate() != nil || seen[key] {
			return ErrInvalid
		}
		seen[key] = true
		bytes += len(document.ID) + len(document.Locale) + len(document.Title) + len(document.Body)
		if bytes > MaximumCorpusBytes {
			return ErrInvalid
		}
	}
	return nil
}

func field(digest hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = io.WriteString(digest, value)
}

// Digest binds exact content, identity, version and audience; input order is irrelevant.
func (corpus Corpus) Digest() ([32]byte, error) {
	if corpus.Validate() != nil {
		return [32]byte{}, ErrInvalid
	}
	documents := append([]Document(nil), corpus.Documents...)
	defer clear(documents)
	sort.Slice(documents, func(i, j int) bool {
		if documents[i].ID != documents[j].ID {
			return documents[i].ID < documents[j].ID
		}
		return documents[i].Locale < documents[j].Locale
	})
	digest := sha256.New()
	field(digest, "stackplore-runbook-corpus-v1")
	field(digest, corpus.ID)
	var version [8]byte
	binary.BigEndian.PutUint64(version[:], uint64(corpus.Version))
	_, _ = digest.Write(version[:])
	field(digest, string(corpus.Audience))
	for _, document := range documents {
		for _, value := range []string{document.ID, document.Locale, document.Title, document.Body} {
			field(digest, value)
		}
	}
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

// Query and Snippet are pure bounds; the repository must authorize approved versions.
func Query(value string, limit int) error {
	if !text(value, MaximumQueryBytes, false) || limit < 1 || limit > MaximumResults {
		return ErrInvalid
	}
	return nil
}

func Snippet(body string) (string, error) {
	if !text(body, MaximumDocumentBytes, true) {
		return "", ErrInvalid
	}
	if len(body) <= MaximumSnippetBytes {
		return body, nil
	}
	end := MaximumSnippetBytes
	for !utf8.RuneStart(body[end]) {
		end--
	}
	return body[:end], nil
}
