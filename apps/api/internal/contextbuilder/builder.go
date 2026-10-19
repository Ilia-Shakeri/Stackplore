package contextbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// Build performs no I/O. The caller must fence authority again before transmission.
func Build(ctx context.Context, request Request) (Bundle, error) {
	if ctx == nil {
		return Bundle{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}
	if err := validateScope(request.Scope); err != nil {
		return Bundle{}, err
	}
	if request.Destination != Local && request.Destination != Remote || request.Status.Validate() != nil || !validTime(request.Status.ObservedAt) || request.Status.ObservedAt.After(request.Scope.ObservedAt) {
		return Bundle{}, ErrInvalid
	}
	if request.Status.State == domain.AvailabilityPermissionDenied {
		return Bundle{}, ErrDenied
	}
	if request.Logs != nil && (request.Destination != Local || !request.LogConsent) {
		return Bundle{}, ErrDenied
	}
	maximum, bytes := request.RecordLimit, request.ByteLimit
	if maximum == 0 {
		maximum = MaximumRecords
	}
	if bytes == 0 {
		bytes = MaximumBytes
	}
	if maximum < 1 || maximum > MaximumRecords || bytes < 1 || bytes > MaximumBytes {
		return Bundle{}, ErrInvalid
	}
	count := len(request.Resources) + len(request.Health) + len(request.Metrics) + len(request.Events) + len(request.Relationships)
	if count > maximum {
		return Bundle{}, ErrBudget
	}
	if !request.Status.AllowsData() && (count > 0 || request.Logs != nil) {
		return Bundle{}, ErrInvalid
	}
	result := projection{ctx: ctx, request: request, maximum: maximum, allowed: make(map[domain.ResourceID]domain.ResourceRef), seen: map[string]bool{}}
	for _, resource := range request.Scope.Resources {
		result.allowed[resource.ID] = resource
	}
	result.bundle.document = document{
		Version: 1, SourceReference: fingerprint("source", string(request.Scope.Source.SourceID)), Provider: request.Scope.Source.Provider,
		ConfigurationVersion: request.Scope.ConfigurationVersion, PolicyVersion: request.Scope.PolicyVersion, RestoreGeneration: request.Scope.RestoreGeneration,
		Range: timeRange{Start: request.Scope.Range.Start, End: request.Scope.Range.End}, Quality: result.quality(request.Status), MoreAvailable: request.MoreAvailable, Records: []record{},
	}
	for _, project := range []func() error{result.resources, result.health, result.metrics, result.events, result.relationships, result.logs} {
		if err := ctx.Err(); err != nil {
			return Bundle{}, err
		}
		if err := project(); err != nil {
			return Bundle{}, err
		}
	}
	if request.Destination == Remote {
		result.remoteAliases()
	}
	encoded, err := result.bundle.JSON()
	if err != nil {
		return Bundle{}, ErrInvalid
	}
	if len(encoded) > bytes {
		return Bundle{}, ErrBudget
	}
	return result.bundle, nil
}

func (bundle Bundle) JSON() ([]byte, error) {
	if bundle.document.Version != 1 {
		return nil, ErrInvalid
	}
	return json.Marshal(bundle.document)
}

func validateScope(scope Scope) error {
	if scope.RunID.Validate() != nil || scope.Source.Validate() != nil || scope.ConfigurationVersion < 1 || scope.PolicyVersion < 1 || scope.RestoreGeneration < 0 || scope.Range.ValidateLookback(domain.MaxMetricLookback) != nil || !validTime(scope.Range.Start) || !validTime(scope.Range.End) || !validTime(scope.ObservedAt) || scope.Range.End.After(scope.ObservedAt) || len(scope.Resources) < 1 || len(scope.Resources) > domain.MaxMetricTargets {
		return ErrInvalid
	}
	seen := map[domain.ResourceID]bool{}
	for _, resource := range scope.Resources {
		if resource.Validate() != nil || !scope.Source.Matches(resource) || seen[resource.ID] {
			return ErrInvalid
		}
		seen[resource.ID] = true
	}
	return nil
}

func validTime(at time.Time) bool {
	return !at.IsZero() && at.Location() == time.UTC && at.Year() >= 1970 && at.Year() <= 9999 && at.Nanosecond()%int(time.Microsecond) == 0
}

func fingerprint(kind, value string) string {
	digest := sha256.Sum256([]byte("stackplore-evidence-v1\x00" + kind + "\x00" + value))
	return "ev_" + hex.EncodeToString(digest[:])
}

// Remote references are scoped to one server-owned run, never stable estate IDs.
// They grant no access; citation resolution still requires fresh local authority.
func (projection *projection) remoteAliases() {
	alias := func(kind, value string) string {
		return fingerprint("remote_"+kind, string(projection.request.Scope.RunID)+"\x00"+value)
	}
	projection.bundle.document.SourceReference = alias("source", string(projection.request.Scope.Source.SourceID))
	for index := range projection.bundle.document.Records {
		item := &projection.bundle.document.Records[index]
		if item.ResourceID != "" {
			item.ResourceID = alias("resource", item.ResourceID)
		}
		if item.VersionReference != "" {
			item.VersionReference = alias("version", item.VersionReference)
		}
		if item.Relationship != nil {
			item.Relationship.From = alias("resource", item.Relationship.From)
			item.Relationship.To = alias("resource", item.Relationship.To)
		}
	}
}

func (projection *projection) quality(status domain.ObservationStatus) quality {
	return quality{Availability: status.State, Completeness: status.Completeness, ObservedAt: status.ObservedAt, Synthetic: projection.request.Scope.Source.Provider == domain.ProviderDemo}
}

func (projection *projection) status(status domain.ObservationStatus) error {
	if status.Validate() != nil || !validTime(status.ObservedAt) || status.ObservedAt.After(projection.request.Scope.ObservedAt) {
		return ErrInvalid
	}
	if status.State == domain.AvailabilityPermissionDenied {
		return ErrDenied
	}
	return nil
}

func (projection *projection) contains(id domain.ResourceID) bool {
	_, exists := projection.allowed[id]
	return exists
}

func (projection *projection) inRange(at time.Time) bool {
	return validTime(at) && !at.Before(projection.request.Scope.Range.Start) && at.Before(projection.request.Scope.Range.End)
}

func (projection *projection) add(kind, identity string, item record) error {
	if err := projection.ctx.Err(); err != nil {
		return err
	}
	if len(projection.bundle.document.Records) >= projection.maximum {
		return ErrBudget
	}
	key := kind + "\x00" + identity
	if projection.seen[key] {
		return ErrInvalid
	}
	projection.seen[key] = true
	scope := projection.request.Scope
	item.Kind = kind
	encoded, err := json.Marshal(item)
	if err != nil {
		return ErrInvalid
	}
	item.CitationID = fingerprint(kind, string(scope.RunID)+"\x00"+string(scope.Source.SourceID)+"\x00"+strconv.FormatInt(scope.ConfigurationVersion, 10)+"\x00"+strconv.FormatInt(scope.PolicyVersion, 10)+"\x00"+strconv.FormatInt(scope.RestoreGeneration, 10)+"\x00"+scope.Range.Start.Format(time.RFC3339Nano)+"\x00"+scope.Range.End.Format(time.RFC3339Nano)+"\x00"+identity+"\x00"+string(encoded))
	projection.bundle.document.Records = append(projection.bundle.document.Records, item)
	projection.bundle.references = append(projection.bundle.references, CitationReference{ID: item.CitationID, Kind: kind, ObjectID: identity, Source: scope.Source, ConfigurationVersion: scope.ConfigurationVersion, PolicyVersion: scope.PolicyVersion, RestoreGeneration: scope.RestoreGeneration, ResourceID: domain.ResourceID(item.ResourceID), ObservedAt: item.Quality.ObservedAt, Range: scope.Range})
	return nil
}

func (projection *projection) resources() error {
	for _, snapshot := range projection.request.Resources {
		if err := projection.status(snapshot.Status); err != nil {
			return err
		}
		if snapshot.Validate() != nil || !projection.contains(snapshot.Resource.Ref.ID) || !projection.request.Scope.Source.Matches(snapshot.Resource.Ref) || snapshot.Resource.Synthetic != projection.bundle.document.Quality.Synthetic {
			return ErrInvalid
		}
		state := domain.HealthUnknown
		if snapshot.Status.AllowsData() && snapshot.Status.Completeness == domain.CompletenessComplete && snapshot.Health != "" {
			state = snapshot.Health
		}
		version := ""
		if snapshot.Resource.Version != "" {
			version = fingerprint("version", snapshot.Resource.Version)
		}
		if err := projection.add("resource", string(snapshot.Resource.Ref.ID), record{ResourceID: string(snapshot.Resource.Ref.ID), Quality: projection.quality(snapshot.Status), Health: state, VersionReference: version}); err != nil {
			return err
		}
	}
	return nil
}

func (projection *projection) health() error {
	for _, snapshot := range projection.request.Health {
		if err := projection.status(snapshot.Status); err != nil {
			return err
		}
		if snapshot.Validate() != nil || !projection.contains(snapshot.ResourceID) {
			return ErrInvalid
		}
		for _, evidence := range snapshot.Evidence {
			if evidence.Synthetic != projection.bundle.document.Quality.Synthetic || !validTime(evidence.ObservedAt) {
				return ErrInvalid
			}
		}
		if err := projection.add("health", string(snapshot.ResourceID), record{ResourceID: string(snapshot.ResourceID), Quality: projection.quality(snapshot.Status), Health: snapshot.InventoryState()}); err != nil {
			return err
		}
	}
	return nil
}

func (projection *projection) metrics() error {
	for _, sample := range projection.request.Metrics {
		if sample.Validate() != nil || !projection.contains(sample.ResourceID) || !projection.inRange(sample.SampledAt) || sample.Synthetic != projection.bundle.document.Quality.Synthetic {
			return ErrInvalid
		}
		item := record{ResourceID: string(sample.ResourceID), Quality: projection.quality(projection.request.Status), Metric: &metricFact{Name: sample.Name, Unit: sample.Unit, Value: sample.Value, SampledAt: sample.SampledAt, IntervalSeconds: int64(sample.Interval / time.Second)}}
		if err := projection.add("metric", string(sample.ID), item); err != nil {
			return err
		}
	}
	return nil
}

func (projection *projection) events() error {
	for _, event := range projection.request.Events {
		if event.Validate() != nil || event.Provider != projection.request.Scope.Source.Provider || event.SourceID != projection.request.Scope.Source.SourceID || !projection.inRange(event.OccurredAt) || !validTime(event.ObservedAt) || event.ObservedAt.After(projection.request.Scope.ObservedAt) || event.Synthetic != projection.bundle.document.Quality.Synthetic || event.ResourceID == nil || !projection.contains(*event.ResourceID) {
			return ErrInvalid
		}
		if event.EnvironmentID != nil && *event.EnvironmentID != projection.allowed[*event.ResourceID].Identity.EnvironmentID {
			return ErrInvalid
		}
		item := record{ResourceID: string(*event.ResourceID), Quality: projection.quality(projection.request.Status), Event: &eventFact{Severity: event.Severity, OccurredAt: event.OccurredAt}}
		item.Quality.ObservedAt = event.ObservedAt
		if err := projection.add("event", string(event.ID), item); err != nil {
			return err
		}
	}
	return nil
}

func (projection *projection) relationships() error {
	for _, relationship := range projection.request.Relationships {
		if relationship.Validate() != nil || !projection.contains(relationship.From) || !projection.contains(relationship.To) || !validTime(relationship.ObservedAt) || relationship.ObservedAt.After(projection.request.Scope.ObservedAt) {
			return ErrInvalid
		}
		evidence := make([]relationshipEvidence, 0, len(relationship.Evidence))
		for _, item := range relationship.Evidence {
			if item.Provider != projection.request.Scope.Source.Provider || item.SourceID != projection.request.Scope.Source.SourceID || item.Synthetic != projection.bundle.document.Quality.Synthetic || !validTime(item.ObservedAt) {
				return ErrInvalid
			}
			evidence = append(evidence, relationshipEvidence{Provenance: item.Provenance, Certainty: item.Certainty, Confidence: item.Confidence})
		}
		item := record{Quality: projection.quality(projection.request.Status), Relationship: &relationshipFact{From: string(relationship.From), To: string(relationship.To), Kind: relationship.Kind, Direction: relationship.Direction, Evidence: evidence}}
		item.Quality.ObservedAt = relationship.ObservedAt
		if err := projection.add("relationship", string(relationship.ID), item); err != nil {
			return err
		}
	}
	return nil
}

func (projection *projection) logs() error {
	if projection.request.Logs == nil {
		return nil
	}
	query, page, valid := projection.request.Logs.Snapshot()
	if !valid {
		return ErrInvalid
	}
	if err := projection.status(page.Status); err != nil {
		return err
	}
	if page.ValidateFor(query) != nil || query.Source != projection.request.Scope.Source || query.Range.Start.Before(projection.request.Scope.Range.Start) || query.Range.End.After(projection.request.Scope.Range.End) || query.Range.End.Sub(query.Range.Start) > MaximumLogWindow || query.LineLimit > MaximumLogLines || query.ByteLimit > MaximumLogBytes {
		return ErrInvalid
	}
	for _, target := range query.Targets {
		if !projection.contains(target.Resource.ID) {
			return ErrInvalid
		}
	}
	projection.bundle.document.MoreAvailable = projection.bundle.document.MoreAvailable || page.Page.HasMore
	for _, line := range page.Records {
		if !projection.contains(line.ResourceID) || line.Synthetic != projection.bundle.document.Quality.Synthetic || !projection.inRange(line.Timestamp) {
			return ErrInvalid
		}
		message, truncated := line.Message, line.Truncated
		if len(message) > MaximumLogLineBytes {
			message, truncated = message[:MaximumLogLineBytes], true
			for !utf8.ValidString(message) {
				message = message[:len(message)-1]
			}
		}
		item := record{ResourceID: string(line.ResourceID), Quality: projection.quality(page.Status), Log: &logFact{Timestamp: line.Timestamp, Severity: line.Severity, Stream: line.Stream, Message: message, Redacted: line.Redacted, Truncated: truncated}}
		if err := projection.add("log", string(line.ID), item); err != nil {
			return err
		}
	}
	return nil
}
