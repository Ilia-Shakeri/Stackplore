package domain

import (
	"sort"
	"testing"
	"time"
)

func TestPageModelsRequireBoundedMatchingLimits(t *testing.T) {
	request := PageRequest{Limit: MaxPageSize}
	if err := request.Validate(); err != nil {
		t.Fatalf("PageRequest.Validate() error = %v", err)
	}
	requireInvalid(t, (PageRequest{Limit: MaxPageSize + 1}).Validate())
	page := PageInfo{Limit: 25, HasMore: true, NextCursor: "bmV4dA"}
	if err := page.Validate(); err != nil {
		t.Fatalf("PageInfo.Validate() error = %v", err)
	}
	page.NextCursor = ""
	requireInvalid(t, page.Validate())
	page = PageInfo{Limit: 0}
	requireInvalid(t, page.Validate())

	request = PageRequest{Limit: 25, Cursor: "Y3VycmVudA"}
	page = PageInfo{Limit: 25, HasMore: true, NextCursor: request.Cursor}
	requireInvalid(t, page.ValidateFor(request))
	page.NextCursor = "bmV4dA"
	if err := page.ValidateFor(request); err != nil {
		t.Fatalf("PageInfo.ValidateFor() error = %v", err)
	}
	requireInvalid(t, PageCursor("bmV4dA\r\n").Validate())
	requireInvalid(t, PageCursor("Zh").Validate())
}

func TestInventoryPageRejectsWrongSourceAndLimit(t *testing.T) {
	query := InventoryQuery{
		Source: fixtureProvider(), SortField: ResourceSortName, SortDirection: SortAscending, Page: PageRequest{Limit: 2},
	}
	snapshot := ResourceSnapshot{
		Resource: Resource{Ref: fixtureResourceRef(t, "service-alpha"), DisplayName: "Service alpha", Synthetic: true},
		Status:   fixtureStatus(),
	}
	page := ResourcePage{Status: fixtureStatus(), Resources: []ResourceSnapshot{snapshot}, Page: fixturePage(2)}
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("ResourcePage.ValidateFor() error = %v", err)
	}
	page.Page.Limit = 1
	requireInvalid(t, page.ValidateFor(query))
	otherIdentity := fixtureIdentity("service-other")
	otherIdentity.SourceID = "source_other"
	otherRef, err := NewResourceRef(otherIdentity)
	if err != nil {
		t.Fatalf("NewResourceRef() error = %v", err)
	}
	page.Page.Limit = 2
	page.Resources[0].Resource.Ref = otherRef
	requireInvalid(t, page.ValidateFor(query))
	page.Resources[0] = snapshot
	page.Resources[0].Status.ObservedAt = page.Status.ObservedAt.Add(time.Nanosecond)
	requireInvalid(t, page.ValidateFor(query))
}

func TestInventoryPageEnforcesFiltersAndSort(t *testing.T) {
	first := fixtureResourceRef(t, "service-alpha")
	second := fixtureResourceRef(t, "service-beta")
	query := InventoryQuery{
		Source: fixtureProvider(),
		Filter: ResourceFilter{
			Kinds:  []ResourceKind{"demo.service"},
			Scopes: []Scope{{Kind: ScopeGlobal}},
		},
		SortField: ResourceSortName, SortDirection: SortAscending, Page: PageRequest{Limit: 2},
	}
	page := ResourcePage{
		Status: fixtureStatus(),
		Resources: []ResourceSnapshot{
			{Resource: Resource{Ref: first, DisplayName: "Alpha"}, Status: fixtureStatus()},
			{Resource: Resource{Ref: second, DisplayName: "Beta"}, Status: fixtureStatus()},
		},
		Page: fixturePage(2),
	}
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("ResourcePage.ValidateFor() error = %v", err)
	}

	page.Resources[0], page.Resources[1] = page.Resources[1], page.Resources[0]
	requireInvalid(t, page.ValidateFor(query))
	page.Resources[0], page.Resources[1] = page.Resources[1], page.Resources[0]

	otherIdentity := fixtureIdentity("service-other-kind")
	otherIdentity.Kind = "demo.worker"
	otherRef, err := NewResourceRef(otherIdentity)
	if err != nil {
		t.Fatalf("NewResourceRef() error = %v", err)
	}
	page.Resources[1].Resource.Ref = otherRef
	requireInvalid(t, page.ValidateFor(query))

	page.Resources[1].Resource.Ref = second
	otherIdentity = fixtureIdentity("service-other-scope")
	otherIdentity.Scope = Scope{Kind: ScopeProject, Name: "sample"}
	otherRef, err = NewResourceRef(otherIdentity)
	if err != nil {
		t.Fatalf("NewResourceRef() error = %v", err)
	}
	page.Resources[1].Resource.Ref = otherRef
	requireInvalid(t, page.ValidateFor(query))
}

func TestInventoryPageEnforcesSearchAndAscendingIDTieBreak(t *testing.T) {
	first := fixtureResourceRef(t, "search-alpha")
	second := fixtureResourceRef(t, "search-beta")
	query := InventoryQuery{
		Source: fixtureProvider(), Filter: ResourceFilter{Search: "ALPHA"},
		SortField: ResourceSortName, SortDirection: SortAscending, Page: PageRequest{Limit: 1},
	}
	page := ResourcePage{
		Status: fixtureStatus(),
		Resources: []ResourceSnapshot{{
			Resource: Resource{Ref: first, DisplayName: "Service alpha"}, Status: fixtureStatus(),
		}},
		Page: fixturePage(1),
	}
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("searched ResourcePage.ValidateFor() error = %v", err)
	}
	page.Resources[0].Resource.DisplayName = "Service beta"
	requireInvalid(t, page.ValidateFor(query))

	query.Filter.Search = ""
	query.SortDirection = SortDescending
	query.Page.Limit = 2
	page.Resources = []ResourceSnapshot{
		{Resource: Resource{Ref: first, DisplayName: "Same"}, Status: fixtureStatus()},
		{Resource: Resource{Ref: second, DisplayName: "Same"}, Status: fixtureStatus()},
	}
	page.Page = fixturePage(2)
	sort.Slice(page.Resources, func(left, right int) bool {
		return page.Resources[left].Resource.Ref.ID < page.Resources[right].Resource.Ref.ID
	})
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("tied ResourcePage.ValidateFor() error = %v", err)
	}
	page.Resources[0], page.Resources[1] = page.Resources[1], page.Resources[0]
	requireInvalid(t, page.ValidateFor(query))
}

func TestResourceDetailBindsRequestedSourceAndResource(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := ResourceDetailQuery{Source: fixtureProvider(), ResourceID: resource.ID}
	snapshot := ResourceSnapshot{
		Resource: Resource{Ref: resource, DisplayName: "Service alpha", Synthetic: true},
		Status:   fixtureStatus(),
	}
	if err := snapshot.ValidateFor(query); err != nil {
		t.Fatalf("ResourceSnapshot.ValidateFor() error = %v", err)
	}

	query.ResourceID = fixtureResourceRef(t, "service-beta").ID
	requireInvalid(t, snapshot.ValidateFor(query))
	query.ResourceID = resource.ID
	query.Source.SourceID = "source_other"
	requireInvalid(t, snapshot.ValidateFor(query))
}

func TestEnvironmentPageRejectsWrongProvider(t *testing.T) {
	query := EnvironmentQuery{Source: fixtureProvider(), Page: PageRequest{Limit: 2}}
	environment := Environment{
		ID: "env_primary", SourceID: "source_demo", Provider: ProviderDemo, Name: "Primary", Status: fixtureStatus(), Synthetic: true,
	}
	page := EnvironmentPage{Status: fixtureStatus(), Environments: []Environment{environment}, Page: fixturePage(2)}
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("EnvironmentPage.ValidateFor() error = %v", err)
	}
	page.Environments[0].Provider = ProviderDocker
	requireInvalid(t, page.ValidateFor(query))
	page.Environments[0] = environment
	page.Environments[0].Status.ObservedAt = page.Status.ObservedAt.Add(time.Nanosecond)
	requireInvalid(t, page.ValidateFor(query))
}

func TestCursorPagesUseStableFixedOrders(t *testing.T) {
	environmentQuery := EnvironmentQuery{Source: fixtureProvider(), Page: PageRequest{Limit: 2}}
	environmentPage := EnvironmentPage{
		Status: fixtureStatus(),
		Environments: []Environment{
			{ID: "env_a", SourceID: "source_demo", Provider: ProviderDemo, Name: "A", Status: fixtureStatus()},
			{ID: "env_b", SourceID: "source_demo", Provider: ProviderDemo, Name: "B", Status: fixtureStatus()},
		},
		Page: fixturePage(2),
	}
	if err := environmentPage.ValidateFor(environmentQuery); err != nil {
		t.Fatalf("EnvironmentPage.ValidateFor() error = %v", err)
	}
	environmentPage.Environments[0], environmentPage.Environments[1] = environmentPage.Environments[1], environmentPage.Environments[0]
	requireInvalid(t, environmentPage.ValidateFor(environmentQuery))

	first := fixtureResourceRef(t, "service-a")
	second := fixtureResourceRef(t, "service-b")
	third := fixtureResourceRef(t, "service-c")
	firstRelationship, err := NewRelationship(first.ID, second.ID, RelationshipDependsOn, RelationshipSourceToTarget, []RelationshipEvidence{{
		ID: "evidence_a", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceDeclaredDependency,
		Certainty: EvidenceExplicit, Confidence: 1, ObservedAt: fixtureTime(),
	}})
	if err != nil {
		t.Fatalf("NewRelationship() first error = %v", err)
	}
	secondRelationship, err := NewRelationship(first.ID, third.ID, RelationshipDependsOn, RelationshipSourceToTarget, []RelationshipEvidence{{
		ID: "evidence_b", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceDeclaredDependency,
		Certainty: EvidenceExplicit, Confidence: 1, ObservedAt: fixtureTime(),
	}})
	if err != nil {
		t.Fatalf("NewRelationship() second error = %v", err)
	}
	relationships := []Relationship{firstRelationship, secondRelationship}
	sort.Slice(relationships, func(left, right int) bool { return relationships[left].ID < relationships[right].ID })
	topologyQuery := TopologyQuery{Source: fixtureProvider(), Page: PageRequest{Limit: 2}}
	relationshipPage := RelationshipPage{Status: fixtureStatus(), Relationships: relationships, Page: fixturePage(2)}
	if err := relationshipPage.ValidateFor(topologyQuery); err != nil {
		t.Fatalf("RelationshipPage.ValidateFor() error = %v", err)
	}
	relationshipPage.Relationships[0], relationshipPage.Relationships[1] = relationshipPage.Relationships[1], relationshipPage.Relationships[0]
	requireInvalid(t, relationshipPage.ValidateFor(topologyQuery))

	eventQuery := EventQuery{
		Source: fixtureProvider(), Range: TimeRange{Start: fixtureTime().Add(-time.Minute), End: fixtureTime().Add(time.Minute)},
		Page: PageRequest{Limit: 2},
	}
	eventPage := EventPage{
		Status: fixtureStatus(),
		Events: []InfrastructureEvent{
			{ID: "event_a", Provider: ProviderDemo, SourceID: "source_demo", Kind: "resource.changed", Severity: SeverityInfo, ReasonCode: "resource.updated", OccurredAt: fixtureTime(), ObservedAt: fixtureTime()},
			{ID: "event_b", Provider: ProviderDemo, SourceID: "source_demo", Kind: "resource.changed", Severity: SeverityInfo, ReasonCode: "resource.updated", OccurredAt: fixtureTime(), ObservedAt: fixtureTime()},
		},
		Page: fixturePage(2),
	}
	if err := eventPage.ValidateFor(eventQuery); err != nil {
		t.Fatalf("EventPage.ValidateFor() error = %v", err)
	}
	eventPage.Events[0], eventPage.Events[1] = eventPage.Events[1], eventPage.Events[0]
	requireInvalid(t, eventPage.ValidateFor(eventQuery))
}

func TestLogPageEnforcesTargetsRangeAndBytes(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := LogReadQuery{
		Source: fixtureProvider(), Targets: []LogTarget{{Resource: resource, Container: "web"}},
		Range:     TimeRange{Start: fixtureTime().Add(-time.Hour), End: fixtureTime().Add(time.Second)},
		LineLimit: 2, ByteLimit: 64,
	}
	record := LogRecord{
		ID: "log_one", ResourceID: resource.ID, Container: "web", Stream: LogStreamStdout,
		Sequence: 0, Severity: LogSeverityInfo, Timestamp: fixtureTime(), Message: "ready", Synthetic: true,
	}
	page := LogPage{Status: fixtureStatus(), Records: []LogRecord{record}, Page: fixturePage(2)}
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("LogPage.ValidateFor() error = %v", err)
	}
	page.Records[0].Container = "worker"
	requireInvalid(t, page.ValidateFor(query))
	page.Records[0] = record
	page.Records[0].Message = "line one\nline two"
	requireInvalid(t, page.ValidateFor(query))
	page.Records[0] = record
	page.Records[0].Message = "safe\x1b[31m"
	requireInvalid(t, page.ValidateFor(query))
	page.Records[0] = record
	page.Records[0].Message = "safe\u202Etext"
	requireInvalid(t, page.ValidateFor(query))
	page.Records[0] = record
	page.Records[0].Message = "safe\u2028text"
	requireInvalid(t, page.ValidateFor(query))
	page.Records[0] = record
	page.Records[0].Timestamp = query.Range.End
	requireInvalid(t, page.ValidateFor(query))
}

func TestLogQueryRejectsAmbiguousCurrentAndPreviousTargets(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := LogReadQuery{
		Source: fixtureProvider(),
		Targets: []LogTarget{
			{Resource: resource, Container: "web"},
			{Resource: resource, Container: "web", Previous: true},
		},
		Range: TimeRange{Start: fixtureTime().Add(-time.Hour), End: fixtureTime()}, LineLimit: 2, ByteLimit: 64,
	}
	requireInvalid(t, query.Validate())
}

func TestStreamQueriesEnforceDeterministicReplayWindows(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	logQuery := LogTailQuery{
		Source: fixtureProvider(), Targets: []LogTarget{{Resource: resource, Container: "web"}},
		Since: fixtureTime().Add(-MaxLogLookback), RequestedAt: fixtureTime(), BufferSize: 1, MaxLineBytes: 64,
	}
	if err := logQuery.Validate(); err != nil {
		t.Fatalf("LogTailQuery.Validate() error = %v", err)
	}
	logQuery.Since = logQuery.Since.Add(-time.Nanosecond)
	requireInvalid(t, logQuery.Validate())
	logQuery.Since = fixtureTime().Add(time.Nanosecond)
	requireInvalid(t, logQuery.Validate())

	eventQuery := EventWatchQuery{
		Source: fixtureProvider(), Since: fixtureTime().Add(-MaxEventLookback), RequestedAt: fixtureTime(), BufferSize: 1,
	}
	if err := eventQuery.Validate(); err != nil {
		t.Fatalf("EventWatchQuery.Validate() error = %v", err)
	}
	eventQuery.Since = eventQuery.Since.Add(-time.Nanosecond)
	requireInvalid(t, eventQuery.Validate())
	eventQuery.Since = fixtureTime()
	eventQuery.RequestedAt = time.Time{}
	requireInvalid(t, eventQuery.Validate())
}

func TestStreamItemsBindToTheirQueries(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	logQuery := LogTailQuery{
		Source: fixtureProvider(), Targets: []LogTarget{{Resource: resource, Container: "web"}},
		Since: fixtureTime().Add(-time.Minute), RequestedAt: fixtureTime(), BufferSize: 1, MaxLineBytes: 64,
	}
	record := LogRecord{
		ID: "log_one", ResourceID: resource.ID, Container: "web", Stream: LogStreamStdout,
		Severity: LogSeverityInfo, Timestamp: fixtureTime().Add(time.Minute), Message: "ready",
	}
	if err := record.ValidateForTail(logQuery); err != nil {
		t.Fatalf("LogRecord.ValidateForTail() error = %v", err)
	}
	logQuery.MaxLineBytes = 4
	requireInvalid(t, record.ValidateForTail(logQuery))
	logQuery.MaxLineBytes = 64
	record.Container = "worker"
	requireInvalid(t, record.ValidateForTail(logQuery))
	record.Container = "web"
	record.Timestamp = logQuery.Since.Add(-time.Nanosecond)
	requireInvalid(t, record.ValidateForTail(logQuery))
	record.Timestamp = fixtureTime().Add(time.Minute)
	record.Sequence = 10
	validator, err := NewLogTailValidator(logQuery)
	if err != nil {
		t.Fatalf("NewLogTailValidator() error = %v", err)
	}
	if err := validator.Validate(record); err != nil {
		t.Fatalf("LogTailValidator.Validate() error = %v", err)
	}
	record.Sequence = 11
	if err := validator.Validate(record); err != nil {
		t.Fatalf("LogTailValidator.Validate() second error = %v", err)
	}
	requireInvalid(t, validator.Validate(record))

	resourceID := resource.ID
	eventQuery := EventWatchQuery{
		Source: fixtureProvider(), Resources: []ResourceRef{resource}, Severities: []Severity{SeverityWarning},
		Since: fixtureTime().Add(-time.Minute), RequestedAt: fixtureTime(), BufferSize: 1,
	}
	event := InfrastructureEvent{
		ID: "event_one", Provider: ProviderDemo, SourceID: "source_demo", ResourceID: &resourceID,
		Kind: "resource.changed", Severity: SeverityWarning, ReasonCode: "resource.updated",
		OccurredAt: fixtureTime().Add(time.Minute), ObservedAt: fixtureTime().Add(time.Minute),
	}
	if err := event.ValidateForWatch(eventQuery); err != nil {
		t.Fatalf("InfrastructureEvent.ValidateForWatch() error = %v", err)
	}
	event.SourceID = "source_other"
	requireInvalid(t, event.ValidateForWatch(eventQuery))
	event.SourceID = "source_demo"
	event.ResourceID = nil
	requireInvalid(t, event.ValidateForWatch(eventQuery))
	event.ResourceID = &resourceID
	event.Severity = SeverityError
	requireInvalid(t, event.ValidateForWatch(eventQuery))
	event.Severity = SeverityWarning
	event.OccurredAt = eventQuery.Since.Add(-time.Nanosecond)
	event.ObservedAt = event.OccurredAt
	requireInvalid(t, event.ValidateForWatch(eventQuery))
}

func TestCompleteHealthResultContainsEveryRequestedResource(t *testing.T) {
	first := fixtureResourceRef(t, "service-alpha")
	second := fixtureResourceRef(t, "service-beta")
	query := HealthQuery{Source: fixtureProvider(), Resources: []ResourceRef{first, second}}
	firstSnapshot := HealthSnapshot{ResourceID: first.ID, State: HealthUnknown, Status: fixtureStatus()}
	secondSnapshot := HealthSnapshot{ResourceID: second.ID, State: HealthUnknown, Status: fixtureStatus()}
	result := HealthResult{Status: fixtureStatus(), Snapshots: []HealthSnapshot{firstSnapshot}}
	requireInvalid(t, result.ValidateFor(query))

	result.Snapshots = append(result.Snapshots, secondSnapshot)
	if err := result.ValidateFor(query); err != nil {
		t.Fatalf("HealthResult.ValidateFor() error = %v", err)
	}

	result.Status.Completeness = CompletenessPartial
	result.Status.ReasonCode = "provider.partial_read"
	result.Snapshots = result.Snapshots[:1]
	if err := result.ValidateFor(query); err != nil {
		t.Fatalf("partial HealthResult.ValidateFor() error = %v", err)
	}
	result.Snapshots[0].Status.ObservedAt = result.Status.ObservedAt.Add(time.Nanosecond)
	requireInvalid(t, result.ValidateFor(query))
}

func TestCurrentMetricResultRejectsUnrequestedAndFutureSamples(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := CurrentMetricQuery{Source: fixtureProvider(), Resources: []ResourceRef{resource}, Names: []MetricName{MetricCPUUsageCores}}
	sample := MetricSample{
		ID: "metric_one", ResourceID: resource.ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores,
		Value: 0.5, SampledAt: fixtureTime(), Interval: time.Second,
	}
	result := CurrentMetricResult{
		Status: fixtureStatus(),
		Sets:   []CurrentMetricSet{{ResourceID: resource.ID, Status: fixtureStatus(), Samples: []MetricSample{sample}}},
	}
	if err := result.ValidateFor(query); err != nil {
		t.Fatalf("CurrentMetricResult.ValidateFor() error = %v", err)
	}
	result.Sets[0].Samples = nil
	requireInvalid(t, result.ValidateFor(query))
	result.Sets[0].Samples = []MetricSample{sample}
	result.Sets[0].Samples[0].SampledAt = fixtureTime().Add(time.Second)
	requireInvalid(t, result.ValidateFor(query))
	result.Sets[0].Samples[0] = sample
	result.Sets[0].Samples[0].Name = MetricMemoryUsageBytes
	result.Sets[0].Samples[0].Unit = MetricUnitBytes
	requireInvalid(t, result.ValidateFor(query))
}

func TestCurrentMetricResultKeepsPerResourceDenial(t *testing.T) {
	first := fixtureResourceRef(t, "service-alpha")
	second := fixtureResourceRef(t, "service-beta")
	query := CurrentMetricQuery{
		Source: fixtureProvider(), Resources: []ResourceRef{first, second}, Names: []MetricName{MetricCPUUsageCores},
	}
	overall := fixtureStatus()
	overall.Completeness = CompletenessPartial
	overall.ReasonCode = "provider.partial_read"
	denied := fixtureStatus()
	denied.State = AvailabilityPermissionDenied
	denied.ReasonCode = "provider.permission_denied"
	sample := MetricSample{
		ID: "metric_one", ResourceID: first.ID, Name: MetricCPUUsageCores, Unit: MetricUnitCores,
		Value: 0.5, SampledAt: fixtureTime(), Interval: time.Second,
	}
	result := CurrentMetricResult{
		Status: overall,
		Sets: []CurrentMetricSet{
			{ResourceID: first.ID, Status: fixtureStatus(), Samples: []MetricSample{sample}},
			{ResourceID: second.ID, Status: denied},
		},
	}
	if err := result.ValidateFor(query); err != nil {
		t.Fatalf("CurrentMetricResult.ValidateFor() error = %v", err)
	}
	deniedSample := sample
	deniedSample.ID = "metric_two"
	deniedSample.ResourceID = second.ID
	result.Sets[1].Samples = []MetricSample{deniedSample}
	requireInvalid(t, result.ValidateFor(query))
}

func TestEventPageRejectsUnscopedWrongSourceAndWrongSeverity(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := EventQuery{
		Source: fixtureProvider(), Resources: []ResourceRef{resource},
		Range:      TimeRange{Start: fixtureTime().Add(-time.Hour), End: fixtureTime().Add(time.Second)},
		Severities: []Severity{SeverityWarning}, Page: PageRequest{Limit: 2},
	}
	environment := EnvironmentID("env_primary")
	resourceID := resource.ID
	event := InfrastructureEvent{
		ID: "event_one", Provider: ProviderDemo, SourceID: "source_demo", EnvironmentID: &environment, ResourceID: &resourceID,
		Kind: "resource.changed", Severity: SeverityWarning, ReasonCode: "resource.updated",
		OccurredAt: fixtureTime(), ObservedAt: fixtureTime(), Synthetic: true,
	}
	page := EventPage{Status: fixtureStatus(), Events: []InfrastructureEvent{event}, Page: fixturePage(2)}
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("EventPage.ValidateFor() error = %v", err)
	}
	page.Events[0].ResourceID = nil
	requireInvalid(t, page.ValidateFor(query))
	page.Events[0] = event
	page.Events[0].SourceID = "source_other"
	requireInvalid(t, page.ValidateFor(query))
	page.Events[0] = event
	page.Events[0].Severity = SeverityError
	requireInvalid(t, page.ValidateFor(query))
	page.Events[0] = event
	page.Events[0].ObservedAt = page.Status.ObservedAt.Add(time.Nanosecond)
	requireInvalid(t, page.ValidateFor(query))
}

func TestRelationshipPageRejectsNewerRelationship(t *testing.T) {
	first := fixtureResourceRef(t, "service-alpha")
	second := fixtureResourceRef(t, "service-beta")
	evidence := []RelationshipEvidence{{
		ID: "evidence_route", Provider: ProviderDemo, SourceID: "source_demo", Provenance: ProvenanceRoute,
		Certainty: EvidenceExplicit, Confidence: 1, ObservedAt: fixtureTime().Add(time.Nanosecond), Synthetic: true,
	}}
	relationship, err := NewRelationship(first.ID, second.ID, RelationshipRoutesTo, RelationshipSourceToTarget, evidence)
	if err != nil {
		t.Fatalf("NewRelationship() error = %v", err)
	}
	query := TopologyQuery{Source: fixtureProvider(), Resources: []ResourceRef{first}, Page: PageRequest{Limit: 1}}
	page := RelationshipPage{Status: fixtureStatus(), Relationships: []Relationship{relationship}, Page: fixturePage(1)}
	requireInvalid(t, page.ValidateFor(query))
	page.Status.ObservedAt = relationship.ObservedAt
	if err := page.ValidateFor(query); err != nil {
		t.Fatalf("RelationshipPage.ValidateFor() error = %v", err)
	}
}

func TestUnavailablePagesCannotCarryData(t *testing.T) {
	resource := fixtureResourceRef(t, "service-alpha")
	query := InventoryQuery{
		Source: fixtureProvider(), SortField: ResourceSortName, SortDirection: SortAscending, Page: PageRequest{Limit: 1},
	}
	status := fixtureStatus()
	status.State = AvailabilityUnavailable
	status.ReasonCode = "provider.offline"
	page := ResourcePage{
		Status: status, Page: fixturePage(1),
		Resources: []ResourceSnapshot{{Resource: Resource{Ref: resource, DisplayName: "Service"}, Status: fixtureStatus()}},
	}
	requireInvalid(t, page.ValidateFor(query))
	page.Resources = nil
	page.Page.HasMore = true
	page.Page.NextCursor = "bmV4dA"
	requireInvalid(t, page.ValidateFor(query))
}
