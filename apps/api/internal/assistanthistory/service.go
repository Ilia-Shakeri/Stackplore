// Package assistanthistory serves bounded owner-private metadata without replaying text.
package assistanthistory

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

const CursorLifetime = 10 * time.Minute
const MaximumCursorBytes = 768

type Service struct {
	repository assistant.HistoryRepository
	key [32]byte
	now func() time.Time
}

func (Service) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "[protected history service]") }
func (Service) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func New(repository assistant.HistoryRepository, key []byte, now func() time.Time) (*Service, error) {
	if repository == nil || len(key) != 32 || now == nil { return nil, assistant.ErrInvalid }
	var secret [32]byte
	copy(secret[:], key)
	if secret == [32]byte{} { return nil, assistant.ErrInvalid }
	return &Service{repository: repository, key: secret, now: now}, nil
}

type Page struct {
	Entries []assistant.HistoryEntry
	NextCursor string
}

func (Page) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "[protected run history page]") }
func (Page) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type cursor struct {
	Anchor int64 `json:"a"`
	Before int64 `json:"b"`
	ID string `json:"i"`
	State assistant.State `json:"s"`
	Limit int `json:"l"`
	Generation int64 `json:"g"`
}

func (service *Service) signature(actor assistant.Actor, body []byte) []byte {
	mac := hmac.New(sha256.New, service.key[:])
	_, _ = io.WriteString(mac, "stackplore-run-history-v1\x00"+actor.UserID+"\x00"+actor.DigestKeyID+"\x00"+strconv.FormatInt(actor.AuthRevision, 10)+"\x00")
	_, _ = mac.Write(actor.SessionDigest[:])
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}

func (service *Service) decode(actor assistant.Actor, state assistant.State, limit int, encoded string, at time.Time) (assistant.HistoryQuery, error) {
	query := assistant.HistoryQuery{State: state, Limit: limit, UpperBound: at, Generation: -1}
	if encoded == "" { return query, query.Validate(at) }
	if len(encoded) > MaximumCursorBytes { return query, assistant.ErrInvalid }
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 { return query, assistant.ErrInvalid }
	body, err := base64.RawURLEncoding.Strict().DecodeString(parts[0])
	if err != nil { return query, assistant.ErrInvalid }
	defer clear(body)
	proof, err := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	if err != nil || len(proof) != sha256.Size || !hmac.Equal(proof, service.signature(actor, body)) { return query, assistant.ErrInvalid }
	var value cursor
	if json.Unmarshal(body, &value) != nil { return query, assistant.ErrInvalid }
	canonical, err := json.Marshal(value)
	defer clear(canonical)
	if err != nil || !bytes.Equal(canonical, body) || value.State != state || value.Limit != limit || value.Generation < 0 { return query, assistant.ErrInvalid }
	query.UpperBound, query.BeforeAt, query.BeforeID, query.Generation = time.UnixMicro(value.Anchor).UTC(), time.UnixMicro(value.Before).UTC(), value.ID, value.Generation
	if query.Validate(at) != nil || query.BeforeID == "" || at.Sub(query.UpperBound) >= CursorLifetime { return query, assistant.ErrInvalid }
	return query, nil
}

func (service *Service) List(ctx context.Context, actor assistant.Actor, state assistant.State, limit int, encoded string) (Page, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || !actor.Valid() { return Page{}, assistant.ErrInvalid }
	at := service.now().UTC().Truncate(time.Microsecond)
	query, err := service.decode(actor, state, limit, encoded, at)
	if err != nil { return Page{}, err }
	work, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := service.repository.ListAssistantHistory(work, actor, query, at)
	if err != nil || work.Err() != nil {
		clear(result.Entries)
		if err == assistant.ErrDenied { return Page{}, assistant.ErrDenied }
		return Page{}, assistant.ErrUnavailable
	}
	if len(result.Entries) > query.Limit || result.Generation < 0 || query.Generation >= 0 && result.Generation != query.Generation || result.HasMore && len(result.Entries) != query.Limit { clear(result.Entries); return Page{}, assistant.ErrUnavailable }
	for index, entry := range result.Entries {
		if entry.Validate(query, at) != nil { clear(result.Entries); return Page{}, assistant.ErrUnavailable }
		if index > 0 {
			prior := result.Entries[index-1]
			if entry.CreatedAt.After(prior.CreatedAt) || entry.CreatedAt.Equal(prior.CreatedAt) && entry.ID >= prior.ID { clear(result.Entries); return Page{}, assistant.ErrUnavailable }
		}
	}
	page := Page{Entries: append([]assistant.HistoryEntry{}, result.Entries...)}
	if result.HasMore {
		last := page.Entries[len(page.Entries)-1]
		body, err := json.Marshal(cursor{Anchor: query.UpperBound.UnixMicro(), Before: last.CreatedAt.UnixMicro(), ID: last.ID, State: query.State, Limit: query.Limit, Generation: result.Generation})
		if err != nil { clear(page.Entries); return Page{}, assistant.ErrUnavailable }
		defer clear(body)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(body)+"."+base64.RawURLEncoding.EncodeToString(service.signature(actor, body))
	}
	return page, nil
}
