package postgres

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const maximumResourceMetadataBytes = 32 * 1024

func encodeResourceMetadata(snapshot domain.ResourceSnapshot) ([]byte, error) {
	if snapshot.Metadata == nil {
		return nil, nil
	}
	if snapshot.Metadata.ValidateFor(snapshot.Resource.Ref.Identity.Provider) != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	encoded, err := json.Marshal(snapshot.Metadata)
	if err != nil || len(encoded) > maximumResourceMetadataBytes {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return encoded, nil
}

func decodeResourceMetadata(encoded []byte, provider domain.ProviderKind) (*domain.ResourceMetadata, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	if len(encoded) > 2*maximumResourceMetadataBytes {
		return nil, domain.ErrRepositoryUnavailable
	}
	var metadata domain.ResourceMetadata
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&metadata) != nil || metadata.ValidateFor(provider) != nil {
		return nil, domain.ErrRepositoryUnavailable
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, domain.ErrRepositoryUnavailable
	}
	return &metadata, nil
}
