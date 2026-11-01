package assistantevidence

import (
	"context"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
)

func TestExplicitReadCapsReachEveryFixedFamilyAndLogReauthorization(t *testing.T) {
	for _, kind := range []assistant.StepKind{assistant.Inventory, assistant.Health, assistant.Metrics, assistant.Events, assistant.Topology, assistant.Logs} {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			f.reads.expectedRecords = 8
			f.request.Kind, f.request.LogConsent = kind, kind == assistant.Logs
			f.request.RecordLimit, f.request.ByteLimit = 8, 8192
			proof, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			if proof.Bundle().Summary().Records != proof.Bundle().RecordCount() {
				t.Fatal("coverage invented records")
			}
			if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, proof); err != nil {
				t.Fatal("bounded proof did not retain its read caps", err)
			}
			if kind == assistant.Logs && (f.reads.lastLogs.LineLimit != 8 || f.reads.lastLogs.ByteLimit != 8192 || f.reads.lastLogs.Cursor != "") {
				t.Fatal("log policy recheck widened the sealed request")
			}
		})
	}
}

func TestInvalidReadCapsFailBeforeAuthorityOrProviderIO(t *testing.T) {
	for _, limits := range [][2]int{{-1, 8192}, {MaximumRecords + 1, 8192}, {8, -1}, {8, contextbuilder.MaximumBytes + 1}} {
		f := newFixture(t)
		f.request.RecordLimit, f.request.ByteLimit = limits[0], limits[1]
		if _, err := f.reader.Read(context.Background(), f.request); err != ErrInvalid || f.gate.checks.Load() != 0 || f.live.calls.Load() != 0 || f.reads.resourceCalls != 0 {
			t.Fatal("invalid caps reached authority or provider I/O")
		}
	}
}
