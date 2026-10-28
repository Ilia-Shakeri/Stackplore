package assistantevidence

import (
	"context"
	"sync"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestTrustedSourceAdvanceKeepsReaderSealAndDeniesOldEvidence(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	evidence, err := f.reader.Read(ctx, f.request)
	if err != nil {
		t.Fatal("collect initial sealed evidence")
	}
	if err := f.reader.AdvanceSourceVersion(f.request.Scope.Source, 4); err != nil {
		t.Fatal("advance trusted source metadata")
	}
	if err := f.reader.AuthorizeEvidence(ctx, f.request.Actor, f.request.Run, evidence); err == nil {
		t.Fatal("old sealed evidence acquired a new source epoch")
	}
	if _, err := f.reader.Read(ctx, f.request); err == nil {
		t.Fatal("old source scope remained readable")
	}
	f.request.Run.SourceVersion, f.request.Scope.ConfigurationVersion = 4, 4
	current, err := f.reader.Read(ctx, f.request)
	if err != nil || current.reader != f.reader {
		t.Fatal("new source epoch lost stable reader ownership")
	}
	if err := f.reader.AuthorizeEvidence(ctx, f.request.Actor, f.request.Run, current); err != nil {
		t.Fatal("new epoch evidence did not retain its authority")
	}
	if err := f.reader.AdvanceSourceVersion(f.request.Scope.Source, 3); err != ErrDenied {
		t.Fatal("source epoch moved backwards")
	}
	if err := f.reader.AdvanceSourceVersion(f.request.Scope.Source, 4); err != nil {
		t.Fatal("same trusted epoch was not idempotent")
	}
}

func TestSourceAdvanceNeverAddsOrSwapsApprovedCapabilities(t *testing.T) {
	f := newFixture(t)
	for _, input := range []struct {
		source  domain.ProviderRef
		version int64
	}{
		{domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "unknown"}, 4},
		{domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: f.request.Scope.Source.SourceID}, 4},
		{f.request.Scope.Source, 0}, {domain.ProviderRef{}, 4},
	} {
		if f.reader.AdvanceSourceVersion(input.source, input.version) == nil {
			t.Fatal("unapproved source advance accepted")
		}
	}
	if (*Reader)(nil).AdvanceSourceVersion(f.request.Scope.Source, 4) != ErrInvalid {
		t.Fatal("nil reader accepted advance")
	}
	if len(f.reader.sources) != 1 || f.reader.sourceVersion(f.reader.sources[f.request.Scope.Source.SourceID]) != 3 {
		t.Fatal("failed advance changed approved sources")
	}
}

func TestSourceVersionConcurrentAdvanceCannotRollBack(t *testing.T) {
	f := newFixture(t)
	var workers sync.WaitGroup
	for version := int64(4); version <= 64; version++ {
		workers.Go(func() {
			_ = f.reader.AdvanceSourceVersion(f.request.Scope.Source, version)
			_ = f.reader.sourceVersion(f.reader.sources[f.request.Scope.Source.SourceID])
		})
	}
	workers.Wait()
	if f.reader.sourceVersion(f.reader.sources[f.request.Scope.Source.SourceID]) != 64 {
		t.Fatal("concurrent advance lost highest version")
	}
}
