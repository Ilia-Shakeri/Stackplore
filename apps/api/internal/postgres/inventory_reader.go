package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const resourceHealthExpression = `CASE WHEN EXISTS(SELECT 1 FROM source_capabilities capability WHERE capability.source_id=resource.source_id AND capability.capability_name='health.read' AND capability.availability IN ('available','stale')) THEN COALESCE((SELECT CASE WHEN health.availability IN ('available','stale') AND health.completeness='complete' THEN health.health_state ELSE 'unknown' END FROM resource_health_current AS health WHERE health.resource_id=resource.id),'unknown') ELSE 'unknown' END`

const observedResourceColumns = `resource.id,resource.source_id,resource.provider,resource.environment_id,
resource.kind,resource.scope_kind,resource.scope_name,resource.native_id,resource.synthetic,
snapshot.display_name,snapshot.resource_version,snapshot.availability,snapshot.completeness,snapshot.observed_at_us,snapshot.reason_code,` + resourceHealthExpression

const activeResourceJoin = ` FROM resources AS resource
JOIN infrastructure_sources AS source ON source.id=resource.source_id
JOIN resource_snapshots AS snapshot ON snapshot.resource_id=resource.id
WHERE source.id=$1 AND source.provider=$2 AND source.deleted_at_us IS NULL
AND resource.retired_at_us IS NULL AND resource.capture_generation>=source.completed_generation`

func (store *observationRead) ListEnvironments(ctx context.Context, query domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.EnvironmentPage{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.EnvironmentPage{}, err
	}
	result := domain.EnvironmentPage{Status: status, Environments: []domain.Environment{}, Page: domain.PageInfo{Limit: query.Page.Limit}}
	if !status.AllowsData() {
		return result, nil
	}
	queryHash := observationQueryHash(query.Source)
	cursor, err := store.decodeObservationCursor(query.Page.Cursor, "environments", queryHash)
	if err != nil {
		return domain.EnvironmentPage{}, err
	}
	after := ""
	if cursor != nil {
		after = cursor.ID
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	rows, err := store.queries.Query(ctx, `SELECT environment.id,environment.display_name,environment.availability,
environment.completeness,environment.observed_at_us,environment.reason_code,environment.synthetic
FROM environments AS environment JOIN infrastructure_sources AS source ON source.id=environment.source_id
WHERE environment.source_id=$1 AND environment.provider=$2 AND environment.retired_at_us IS NULL
AND environment.capture_generation>=source.completed_generation AND environment.id COLLATE "C">$3 COLLATE "C"
ORDER BY environment.id COLLATE "C" LIMIT $4`, string(query.Source.SourceID), string(query.Source.Provider), after, query.Page.Limit+1)
	if err != nil {
		return domain.EnvironmentPage{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var environment domain.Environment
		var availability, completeness string
		var observedAt, synthetic int64
		var reason sql.NullString
		if rows.Scan(&environment.ID, &environment.Name, &availability, &completeness, &observedAt, &reason, &synthetic) != nil {
			return domain.EnvironmentPage{}, domain.ErrRepositoryUnavailable
		}
		environment.SourceID = query.Source.SourceID
		environment.Provider = query.Source.Provider
		environment.Synthetic = synthetic == 1
		environment.Status = scanObservationStatus(availability, completeness, observedAt, reason)
		result.Environments = append(result.Environments, environment)
	}
	if err := rows.Err(); err != nil {
		return domain.EnvironmentPage{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityInventoryList, status, len(result.Environments) > 0)
	if err != nil {
		return domain.EnvironmentPage{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		result.Environments = []domain.Environment{}
		return result, nil
	}
	for index := range result.Environments {
		if status.State == domain.AvailabilityStale && result.Environments[index].Status.AllowsData() {
			result.Environments[index].Status.State = domain.AvailabilityStale
			result.Environments[index].Status.ReasonCode = status.ReasonCode
		}
	}
	if len(result.Environments) > query.Page.Limit {
		result.Environments = result.Environments[:query.Page.Limit]
		result.Page.HasMore = true
		result.Page.NextCursor, err = store.encodeObservationCursor("environments", queryHash, "", string(result.Environments[len(result.Environments)-1].ID))
		if err != nil {
			return domain.EnvironmentPage{}, err
		}
	}
	if result.ValidateFor(query) != nil {
		return domain.EnvironmentPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *observationRead) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if ctx == nil || query.Validate() != nil || !store.available() {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	resource, err := scanObservedResource(store.queries.QueryRow(ctx, `SELECT `+observedResourceColumns+activeResourceJoin+` AND resource.id=$3`, string(query.Source.SourceID), string(query.Source.Provider), string(query.ResourceID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryNotFound
	}
	if err != nil {
		return domain.ResourceSnapshot{}, mapRepositoryReadError(ctx, err)
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.ResourceSnapshot{}, err
	}
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityInventoryDetail, status, true)
	if err != nil {
		return domain.ResourceSnapshot{}, err
	}
	if !status.AllowsData() {
		code := domain.ProviderErrorUnavailable
		if status.State == domain.AvailabilityPermissionDenied {
			code = domain.ProviderErrorPermissionDenied
		}
		if status.State == domain.AvailabilityUnsupported {
			code = domain.ProviderErrorUnsupported
		}
		blocked, _ := domain.NewProviderError("inventory.detail", code, false, domain.ErrRepositoryUnavailable)
		return domain.ResourceSnapshot{}, blocked
	}
	if status.State == domain.AvailabilityStale && resource.Status.AllowsData() {
		resource.Status.State = domain.AvailabilityStale
		resource.Status.ReasonCode = status.ReasonCode
	}
	if resource.ValidateFor(query) != nil {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryUnavailable
	}
	return resource, nil
}

func (store *observationRead) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.ResourcePage{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	if status.AllowsData() {
		var cached bool
		if err := store.queries.QueryRow(ctx, `SELECT EXISTS(SELECT 1`+activeResourceJoin+`)`, string(query.Source.SourceID), string(query.Source.Provider)).Scan(&cached); err != nil {
			return domain.ResourcePage{}, mapRepositoryReadError(ctx, err)
		}
		status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityInventoryList, status, cached)
		if err != nil {
			return domain.ResourcePage{}, err
		}
	}
	result := domain.ResourcePage{Status: status, Resources: []domain.ResourceSnapshot{}, Page: domain.PageInfo{Limit: query.Page.Limit}}
	if !status.AllowsData() {
		return result, nil
	}
	queryBinding := query
	queryBinding.Page = domain.PageRequest{}
	queryHash := observationQueryHash(queryBinding)
	cursor, err := store.decodeObservationCursor(query.Page.Cursor, "resources", queryHash)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	statement := `SELECT ` + observedResourceColumns + activeResourceJoin
	arguments := []any{string(query.Source.SourceID), string(query.Source.Provider)}
	placeholder := func(value any) string {
		arguments = append(arguments, value)
		return "$" + strconv.Itoa(len(arguments))
	}
	for _, filter := range []struct{ column, value string }{
		{"resource.provider", string(query.Filter.Provider)}, {"resource.environment_id", string(query.Filter.EnvironmentID)},
		{"resource.scope_kind", string(query.Filter.ScopeKind)}, {"resource.scope_name", query.Filter.ScopeName},
		{resourceHealthExpression, string(query.Filter.Health)},
	} {
		if filter.value != "" {
			statement += ` AND ` + filter.column + `=` + placeholder(filter.value)
		}
	}
	if query.Filter.Availability != "" {
		column := `snapshot.availability`
		if status.State == domain.AvailabilityStale {
			column = `CASE WHEN snapshot.availability IN ('available','stale') THEN 'stale' ELSE snapshot.availability END`
		}
		statement += ` AND (` + column + `)=` + placeholder(string(query.Filter.Availability))
	}
	if query.Filter.Search != "" {
		statement += ` AND position(lower(` + placeholder(query.Filter.Search) + ` COLLATE pg_catalog."pg_c_utf8") in lower((snapshot.display_name||chr(10)||resource.id||chr(10)||resource.environment_id||chr(10)||resource.kind||chr(10)||resource.scope_kind||chr(10)||resource.scope_name) COLLATE pg_catalog."pg_c_utf8"))>0`
	}
	if len(query.Filter.Kinds) > 0 {
		kinds := make([]string, 0, len(query.Filter.Kinds))
		for _, kind := range query.Filter.Kinds {
			kinds = append(kinds, string(kind))
		}
		statement += ` AND resource.kind=ANY(` + placeholder(kinds) + `::text[])`
	}
	if len(query.Filter.Scopes) > 0 {
		parts := make([]string, 0, len(query.Filter.Scopes))
		for _, scope := range query.Filter.Scopes {
			parts = append(parts, `(resource.scope_kind=`+placeholder(string(scope.Kind))+` AND resource.scope_name=`+placeholder(scope.Name)+`)`)
		}
		statement += ` AND (` + strings.Join(parts, " OR ") + `)`
	}
	orderColumn := `snapshot.display_name COLLATE "C"`
	switch query.SortField {
	case domain.ResourceSortKind:
		orderColumn = `resource.kind COLLATE "C"`
	case domain.ResourceSortObservedAt:
		orderColumn = `snapshot.observed_at_us`
	}
	direction, comparison := "ASC", ">"
	if query.SortDirection == domain.SortDescending {
		direction, comparison = "DESC", "<"
	}
	if cursor != nil {
		var value any
		if query.SortField == domain.ResourceSortObservedAt {
			timestamp, parseErr := strconv.ParseInt(cursor.Value, 10, 64)
			if parseErr != nil || timestamp < 0 {
				return domain.ResourcePage{}, domain.ErrRepositoryInvalidInput
			}
			value = timestamp
		} else {
			value = cursor.Value
		}
		bound := placeholder(value)
		if query.SortField != domain.ResourceSortObservedAt {
			bound += ` COLLATE "C"`
		}
		statement += ` AND (` + orderColumn + comparison + bound + ` OR (` + orderColumn + `=` + bound + ` AND resource.id COLLATE "C">` + placeholder(cursor.ID) + ` COLLATE "C"))`
	}
	statement += ` ORDER BY ` + orderColumn + ` ` + direction + `,resource.id COLLATE "C" ASC LIMIT ` + placeholder(query.Page.Limit+1)
	rows, err := store.queries.Query(ctx, statement, arguments...)
	if err != nil {
		return domain.ResourcePage{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		resource, err := scanObservedResource(rows)
		if err != nil {
			return domain.ResourcePage{}, domain.ErrRepositoryUnavailable
		}
		result.Resources = append(result.Resources, resource)
	}
	if err := rows.Err(); err != nil {
		return domain.ResourcePage{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	result.Status = status
	if !status.AllowsData() {
		result.Resources = []domain.ResourceSnapshot{}
		return result, nil
	}
	for index := range result.Resources {
		if status.State == domain.AvailabilityStale && result.Resources[index].Status.AllowsData() {
			result.Resources[index].Status.State = domain.AvailabilityStale
			result.Resources[index].Status.ReasonCode = status.ReasonCode
		}
	}
	if len(result.Resources) > query.Page.Limit {
		result.Resources = result.Resources[:query.Page.Limit]
		result.Page.HasMore = true
		last := result.Resources[len(result.Resources)-1]
		value := last.Resource.DisplayName
		switch query.SortField {
		case domain.ResourceSortKind:
			value = string(last.Resource.Ref.Identity.Kind)
		case domain.ResourceSortObservedAt:
			value = strconv.FormatInt(last.Status.ObservedAt.UnixMicro(), 10)
		}
		result.Page.NextCursor, err = store.encodeObservationCursor("resources", queryHash, value, string(last.Resource.Ref.ID))
		if err != nil {
			return domain.ResourcePage{}, err
		}
	}
	if result.ValidateFor(query) != nil {
		return domain.ResourcePage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func scanObservedResource(row rowScanner) (domain.ResourceSnapshot, error) {
	var result domain.ResourceSnapshot
	var id, sourceID, provider, environmentID, kind, scopeKind, scopeName, nativeID string
	var synthetic, observedAt int64
	var availability, completeness string
	var reason sql.NullString
	if err := row.Scan(&id, &sourceID, &provider, &environmentID, &kind, &scopeKind, &scopeName, &nativeID, &synthetic,
		&result.Resource.DisplayName, &result.Resource.Version, &availability, &completeness, &observedAt, &reason, &result.Health); err != nil {
		return domain.ResourceSnapshot{}, err
	}
	identity, err := domain.NewResourceIdentity(domain.ProviderKind(provider), domain.SourceID(sourceID), domain.EnvironmentID(environmentID), domain.ResourceKind(kind), domain.Scope{Kind: domain.ScopeKind(scopeKind), Name: scopeName}, domain.NativeResourceID(nativeID))
	if err != nil {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryUnavailable
	}
	result.Resource.Ref = domain.ResourceRef{ID: domain.ResourceID(id), Identity: identity}
	result.Resource.Synthetic = synthetic == 1
	result.Status = scanObservationStatus(availability, completeness, observedAt, reason)
	if result.Validate() != nil {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *observationRead) GetWorkloadReplicas(ctx context.Context, query domain.ResourceDetailQuery) (domain.WorkloadReplicaSnapshot, error) {
	resource, err := store.GetResource(ctx, query)
	if err != nil {
		return domain.WorkloadReplicaSnapshot{}, err
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	var desired, ready, observedAt sql.NullInt64
	var availability, completeness, reason sql.NullString
	err = store.queries.QueryRow(ctx, `SELECT desired_replicas,ready_replicas,replicas_availability,replicas_completeness,replicas_observed_at_us,replicas_reason_code
FROM resource_snapshots WHERE resource_id=$1`, string(query.ResourceID)).Scan(&desired, &ready, &availability, &completeness, &observedAt, &reason)
	if err != nil {
		return domain.WorkloadReplicaSnapshot{}, mapRepositoryReadError(ctx, err)
	}
	status := domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: resource.Status.ObservedAt, ReasonCode: "source.replicas_unavailable"}
	result := domain.WorkloadReplicaSnapshot{Resource: resource.Resource.Ref, Status: status}
	if desired.Valid && ready.Valid && observedAt.Valid && availability.Valid && completeness.Valid {
		result.Desired, result.Ready = uint64(desired.Int64), uint64(ready.Int64)
		result.Status = scanObservationStatus(availability.String, completeness.String, observedAt.Int64, reason)
	}
	if resource.Status.State == domain.AvailabilityStale && result.Status.AllowsData() {
		result.Status.State = domain.AvailabilityStale
		result.Status.ReasonCode = resource.Status.ReasonCode
	}
	if result.ValidateFor(query) != nil {
		return domain.WorkloadReplicaSnapshot{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}
