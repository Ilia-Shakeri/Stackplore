package postgres

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type observationCursor struct {
	Version   int    `json:"v"`
	Kind      string `json:"k"`
	QueryHash string `json:"q"`
	Value     string `json:"s"`
	ID        string `json:"id"`
}

func observationQueryHash(query any) string {
	encoded, _ := json.Marshal(query)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (store *Store) encodeObservationCursor(kind, queryHash, value, id string) (domain.PageCursor, error) {
	payload, err := json.Marshal(observationCursor{Version: 1, Kind: kind, QueryHash: queryHash, Value: value, ID: id})
	if err != nil {
		return "", domain.ErrRepositoryUnavailable
	}
	key := store.snapshotAuthDigestKey()
	defer clear(key[:])
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte("stackplore/observations/cursor/v1\x00"))
	_, _ = mac.Write(payload)
	signed := append(payload, mac.Sum(nil)...)
	encoded := domain.PageCursor(base64.RawURLEncoding.EncodeToString(signed))
	if encoded.Validate() != nil {
		return "", domain.ErrRepositoryUnavailable
	}
	return encoded, nil
}

func (store *Store) decodeObservationCursor(value domain.PageCursor, kind, queryHash string) (*observationCursor, error) {
	if value == "" {
		return nil, nil
	}
	if value.Validate() != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	signed, err := base64.RawURLEncoding.Strict().DecodeString(string(value))
	if err != nil || len(signed) <= sha256.Size {
		return nil, domain.ErrRepositoryInvalidInput
	}
	payload, signature := signed[:len(signed)-sha256.Size], signed[len(signed)-sha256.Size:]
	key := store.snapshotAuthDigestKey()
	defer clear(key[:])
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte("stackplore/observations/cursor/v1\x00"))
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, domain.ErrRepositoryInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var state observationCursor
	if decoder.Decode(&state) != nil || state.Version != 1 || state.Kind != kind || state.QueryHash != queryHash || state.ID == "" || len(state.ID) > 128 {
		return nil, domain.ErrRepositoryInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return &state, nil
}
