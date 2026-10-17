package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var allMetricNames = []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricNetworkReceiveBytesTotal, domain.MetricNetworkSendBytesTotal, domain.MetricRestartCount, domain.MetricCPURequestCores, domain.MetricCPULimitCores, domain.MetricMemoryRequestBytes, domain.MetricMemoryLimitBytes}

func scenarioResources(t *testing.T, provider *Provider) []domain.ResourceRef {
	t.Helper()
	page, err := provider.ListResources(context.Background(), inventoryQuery(20))
	if err != nil {
		t.Fatal(err)
	}
	refs := make([]domain.ResourceRef, len(page.Resources))
	for index, item := range page.Resources {
		refs[index] = item.Resource.Ref
	}
	return refs
}

func TestTwoEnvironmentStoryAndOriginalResourceIdentities(t *testing.T) {
	provider := newProvider(t)
	refs := scenarioResources(t, provider)
	environments, err := provider.ListEnvironments(context.Background(), domain.EnvironmentQuery{Source: inventoryQuery(20).Source, Page: domain.PageRequest{Limit: 1}})
	if err != nil || len(environments.Environments) != 1 || !environments.Page.HasMore {
		t.Fatalf("environment first page: %v", err)
	}
	next, err := provider.ListEnvironments(context.Background(), domain.EnvironmentQuery{Source: inventoryQuery(20).Source, Page: domain.PageRequest{Limit: 1, Cursor: environments.Page.NextCursor}})
	if err != nil || len(next.Environments) != 1 || next.Page.HasMore || next.Environments[0].ID == environments.Environments[0].ID {
		t.Fatalf("environment second page: %v", err)
	}
	states := map[domain.HealthState]int{}
	for _, ref := range refs {
		states[provider.health[ref.ID].State]++
	}
	if len(refs) != 9 || states[domain.HealthHealthy] != 7 || states[domain.HealthDegraded] != 1 || states[domain.HealthUnhealthy] != 1 {
		t.Fatalf("initial story health: %v", states)
	}
	for _, definition := range []struct{ native, kind string }{{"api", "demo.service"}, {"checkout", "demo.service"}, {"gateway", "demo.gateway"}, {"postgres", "demo.database"}, {"redis", "demo.cache"}, {"worker", "demo.worker"}} {
		identity, err := domain.NewResourceIdentity(domain.ProviderDemo, SourceID, EnvironmentID, domain.ResourceKind(definition.kind), domain.Scope{Kind: domain.ScopeProject, Name: "demo-shop"}, domain.NativeResourceID(definition.native))
		if err != nil {
			t.Fatal(err)
		}
		expected, err := domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(refs, expected) {
			t.Fatalf("original identity changed for %s", definition.native)
		}
	}
	for _, relationship := range provider.relationships {
		var from, to domain.NativeResourceID
		for _, ref := range refs {
			if ref.ID == relationship.From {
				from = ref.Identity.NativeID()
			}
			if ref.ID == relationship.To {
				to = ref.Identity.NativeID()
			}
		}
		if from == "checkout" && to == "gateway" {
			t.Fatal("checkout cannot route traffic to the reverse proxy")
		}
		if relationship.Kind == domain.RelationshipRoutesTo && relationship.Evidence[0].Provenance != domain.ProvenanceRoute {
			t.Fatal("route lacks route evidence")
		}
	}
}

func TestMetricFactsIndependentOfTargetsOrderAndNames(t *testing.T) {
	provider := newProvider(t)
	refs := scenarioResources(t, provider)
	query := domain.CurrentMetricQuery{Source: inventoryQuery(20).Source, Resources: refs, Names: allMetricNames}
	whole, err := provider.GetCurrentMetrics(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[domain.MetricSampleID]domain.MetricSample{}
	for _, set := range whole.Sets {
		for _, sample := range set.Samples {
			byID[sample.ID] = sample
		}
	}
	for _, ref := range refs {
		query.Resources = []domain.ResourceRef{ref}
		query.Names = slices.Clone(allMetricNames)
		slices.Reverse(query.Names)
		only, err := provider.GetCurrentMetrics(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		for _, sample := range only.Sets[0].Samples {
			if byID[sample.ID] != sample {
				t.Fatal("subset or metric-order changed a current fact")
			}
		}
		historyQuery := domain.MetricHistoryQuery{Source: query.Source, Resources: []domain.ResourceRef{ref}, Names: query.Names, Range: domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Millisecond)}, PointLimit: 200}
		history, err := provider.GetMetricHistory(context.Background(), historyQuery)
		if err != nil || len(history.Samples) != 108 || history.Status.Completeness != domain.CompletenessComplete {
			t.Fatalf("bounded per-resource history: %v", err)
		}
		for _, sample := range history.Samples {
			if sample.SampledAt.Equal(observedAt) && byID[sample.ID] != sample {
				t.Fatal("current sample differs from latest historical fact")
			}
		}
	}
}

func TestControlledStepsPreservePriorFactsAndReset(t *testing.T) {
	initial := newProvider(t)
	later, err := NewAtStep(2)
	if err != nil {
		t.Fatal(err)
	}
	if later.ScenarioStep() != 2 || !later.ObservationTime().Equal(initial.ObservationTime().Add(10*time.Minute)) {
		t.Fatal("scenario clock does not follow explicit step")
	}
	refs := scenarioResources(t, initial)
	for _, ref := range refs {
		query := domain.MetricHistoryQuery{Source: inventoryQuery(20).Source, Resources: []domain.ResourceRef{ref}, Names: allMetricNames, Range: domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Millisecond)}, PointLimit: 200}
		before, err := initial.GetMetricHistory(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		after, err := later.GetMetricHistory(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before.Samples, after.Samples) {
			t.Fatal("advancing scenario rewrote historical metric facts")
		}
		if later.health[ref.ID].State != domain.HealthHealthy {
			t.Fatal("final step did not recover resources")
		}
	}
	if later.Alerts()[0].State != domain.AlertResolved || initial.Alerts()[0].State != domain.AlertFiring {
		t.Fatal("expected alert episode states differ")
	}
	reset := newProvider(t)
	if !reflect.DeepEqual(initial.resources, reset.resources) || !reflect.DeepEqual(initial.logs, reset.logs) {
		t.Fatal("new process did not reset to packaged initial step")
	}
	for _, step := range []int{-1, 3} {
		if _, err := NewAtStep(step); err == nil {
			t.Fatal("unbounded scenario step accepted")
		}
	}
}

func TestFixtureReadResultsCannotMutateProviderState(t *testing.T) {
	provider := newProvider(t)
	ref := scenarioResources(t, provider)[0]
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	capture.Health[0].Evidence[0].Code = "test.changed"
	capture.Relationships[0].Evidence[0].Confidence = 0
	fresh, err := provider.CaptureInventory(context.Background())
	if err != nil || fresh.Health[0].Evidence[0].Code == "test.changed" || fresh.Relationships[0].Evidence[0].Confidence != 1 {
		t.Fatal("capture exposed mutable fixture slices")
	}
	query := domain.EventQuery{Source: inventoryQuery(20).Source, Range: domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Millisecond)}, Page: domain.PageRequest{Limit: 20}}
	events, err := provider.ListEvents(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	*events.Events[0].ResourceID = ref.ID
	freshEvents, err := provider.ListEvents(context.Background(), query)
	if err != nil || reflect.DeepEqual(events.Events[0], freshEvents.Events[0]) {
		t.Fatal("event read exposed mutable identifier pointers")
	}
	alerts := provider.Alerts()
	*alerts[0].ResourceID = ref.ID
	if *provider.Alerts()[0].ResourceID == ref.ID {
		t.Fatal("alert seed exposed mutable identifier pointers")
	}
}

func TestUnknownTargetsAndUnsupportedPreviousLogsRemainTruthful(t *testing.T) {
	provider := newProvider(t)
	refs := scenarioResources(t, provider)
	identity, err := domain.NewResourceIdentity(domain.ProviderDemo, SourceID, EnvironmentID, "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "demo-shop"}, "missing")
	if err != nil {
		t.Fatal(err)
	}
	missing, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.GetCurrentMetrics(context.Background(), domain.CurrentMetricQuery{Source: inventoryQuery(20).Source, Resources: []domain.ResourceRef{missing}, Names: allMetricNames}); err == nil {
		t.Fatal("invented metric facts for unknown resource")
	}
	query := domain.LogReadQuery{Source: inventoryQuery(20).Source, Targets: []domain.LogTarget{{Resource: refs[0], Previous: true}}, Range: domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Millisecond)}, LineLimit: 20, ByteLimit: 1024}
	logs, err := provider.ReadLogs(context.Background(), query)
	if err != nil || logs.Status.State != domain.AvailabilityUnsupported || len(logs.Records) != 0 || logs.ValidateFor(query) != nil {
		t.Fatal("previous-container logs were fabricated")
	}
}

func TestPackagedFixtureHasNoOutboundOrFilesystemImports(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Clean(entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			name := strings.Trim(imported.Path.Value, `"`)
			if name == "os" || strings.HasPrefix(name, "os/") || name == "net" || strings.HasPrefix(name, "net/") {
				t.Fatalf("packaged fixture imported outbound or filesystem access: %s", name)
			}
		}
	}
}

func TestInitialFixtureSnapshot(t *testing.T) {
	provider := newProvider(t)
	refs := scenarioResources(t, provider)
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	history := []domain.MetricSample{}
	for _, ref := range refs {
		page, err := provider.GetMetricHistory(context.Background(), domain.MetricHistoryQuery{Source: inventoryQuery(20).Source, Resources: []domain.ResourceRef{ref}, Names: allMetricNames, Range: domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Millisecond)}, PointLimit: 200})
		if err != nil {
			t.Fatal(err)
		}
		history = append(history, page.Samples...)
	}
	encoded, err := json.Marshal(struct {
		Version string
		Capture domain.InventoryCapture
		Logs    []domain.LogRecord
		Events  []domain.InfrastructureEvent
		History []domain.MetricSample
		Alerts  []domain.Alert
	}{FixtureVersion, capture, provider.logs, provider.events, history, provider.Alerts()})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	if got := hex.EncodeToString(digest[:]); got != "41ce30de675b647ef3a4b615161913275ca500ce41a1ab0ccdd63b18152dcdad" {
		t.Fatalf("fixture snapshot changed: %s", got)
	}
}
