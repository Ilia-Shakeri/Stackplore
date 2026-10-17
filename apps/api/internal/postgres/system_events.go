package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// appendSystemEvent joins source provenance inside the owning business transaction.
// Only fixed summaries and reason codes enter the viewer timeline; actor, audit,
// confirmation, credential and session data remain in protected history.
func appendSystemEvent(ctx context.Context, tx pgx.Tx, receipt string, source domain.SourceID, environment *domain.EnvironmentID, resource *domain.ResourceID, kind domain.EventKind, severity domain.Severity, reason domain.ReasonCode, summary string, at time.Time) error {
	digest := sha256.Sum256([]byte(receipt))
	id := "event_system_" + hex.EncodeToString(digest[:16])
	result, err := tx.Exec(ctx, `INSERT INTO infrastructure_events
(id,origin_kind,provider,source_id,environment_id,resource_id,kind,severity,reason_code,summary,occurred_at_us,observed_at_us,synthetic)
SELECT $1,'system',provider,id,$3,$4,$5,$6,$7,$8,$9,$9,synthetic
FROM infrastructure_sources WHERE id=$2`, id, string(source), optionalEnvironmentID(environment), optionalResourceID(resource), string(kind), string(severity), string(reason), summary, at.UnixMicro())
	return exactInsertResult(ctx, result.RowsAffected(), err)
}
