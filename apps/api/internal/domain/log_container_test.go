package domain

import (
	"testing"
	"time"
)

func TestLogContainerWildcardKeepsResourceBoundary(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := LogReadQuery{Source: fixtureProvider(), Targets: []LogTarget{{Resource: resource}}, Range: TimeRange{Start: fixtureTime().Add(-time.Minute), End: fixtureTime().Add(time.Second)}, LineLimit: 2, ByteLimit: 64}
	record := LogRecord{ID: "log_named_container", ResourceID: resource.ID, Container: "actual-worker", Stream: LogStreamStdout, Severity: LogSeverityInfo, Timestamp: fixtureTime(), Message: "ready"}
	page := LogPage{Status: fixtureStatus(), Records: []LogRecord{record}, Page: fixturePage(2)}
	if err := page.ValidateFor(query); err != nil {
		t.Fatal(err)
	}
	tail := LogTailQuery{Source: query.Source, Targets: query.Targets, Since: query.Range.Start, RequestedAt: fixtureTime(), BufferSize: 1, MaxLineBytes: 64}
	if err := record.ValidateForTail(tail); err != nil {
		t.Fatal(err)
	}
	query.Targets[0].Container = "other-container"
	tail.Targets = query.Targets
	requireInvalid(t, page.ValidateFor(query))
	requireInvalid(t, record.ValidateForTail(tail))
	query.Targets[0].Container = ""
	query.Targets[0].Resource = fixtureResourceRef(t, "service-beta")
	tail.Targets = query.Targets
	requireInvalid(t, page.ValidateFor(query))
	requireInvalid(t, record.ValidateForTail(tail))
}
