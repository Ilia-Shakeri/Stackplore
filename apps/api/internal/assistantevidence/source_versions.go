package assistantevidence

import (
	"sync"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type sourceConfigurationVersions struct {
	mu     sync.RWMutex
	values map[domain.SourceID]int64
}

// AdvanceSourceVersion accepts only trusted current repository metadata for an
// already approved provider boundary. It never changes read/live capabilities,
// adds a source, renews evidence or gives an old citation a new epoch.
func (reader *Reader) AdvanceSourceVersion(reference domain.ProviderRef, version int64) error {
	if reader == nil || reader.versions == nil || reference.Validate() != nil || version < 1 {
		return ErrInvalid
	}
	source, ok := reader.sources[reference.SourceID]
	if !ok || source.Reference != reference {
		return ErrDenied
	}
	reader.versions.mu.Lock()
	defer reader.versions.mu.Unlock()
	current := max(source.ConfigurationVersion, reader.versions.values[reference.SourceID])
	if version < current {
		return ErrDenied
	}
	reader.versions.values[reference.SourceID] = version
	return nil
}

func (reader *Reader) sourceVersion(source Source) int64 {
	if reader.versions == nil {
		return source.ConfigurationVersion
	}
	reader.versions.mu.RLock()
	defer reader.versions.mu.RUnlock()
	return max(source.ConfigurationVersion, reader.versions.values[source.Reference.SourceID])
}
