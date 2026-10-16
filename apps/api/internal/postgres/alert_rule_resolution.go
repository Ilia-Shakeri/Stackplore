package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Changed rules close their old episode rather than applying new criteria to it.
func resolveRuleAlerts(ctx context.Context, transaction *transaction, ruleID domain.AlertRuleID, at time.Time) error {
	current, err := scanAlert(transaction.transaction.QueryRow(ctx, `SELECT id,rule_id,source_id,environment_id,resource_id,state,severity,evidence_start_at_us,evidence_end_at_us,started_at_us,updated_at_us,acknowledged_at_us,resolved_at_us,summary_code,synthetic FROM alerts WHERE rule_id=$1 AND resolved_at_us IS NULL ORDER BY started_at_us DESC,id LIMIT 1 FOR UPDATE`, string(ruleID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var dedupe string
	var userID *domain.UserID
	if err := transaction.transaction.QueryRow(ctx, `SELECT dedupe_key,acknowledged_by_user_id FROM alerts WHERE id=$1`, string(current.ID)).Scan(&dedupe, &userID); err != nil {
		return err
	}
	previousAt, previousState := current.UpdatedAt, current.State
	if !at.After(previousAt) {
		at = previousAt.Add(time.Microsecond)
	}
	current.State = domain.AlertResolved
	current.ResolvedAt = &at
	current.UpdatedAt = at
	hash := sha256.Sum256([]byte(string(current.ID) + "\x00" + at.Format(time.RFC3339Nano)))
	suffix := hex.EncodeToString(hash[:16])
	mutation := domain.AlertMutation{Alert: current, DedupeKey: dedupe, ExpectedUpdatedAt: &previousAt, ExpectedState: &previousState, AcknowledgedByUserID: userID,
		Transition: &domain.AlertTransition{ID: domain.AlertTransitionID("transition_rule_" + suffix), AlertID: current.ID, From: &previousState, To: domain.AlertResolved, OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, ReasonCode: "alert.rule_changed", EvidenceRange: current.EvidenceRange}}
	return (&transactionRepositories{transaction: transaction}).SaveAlertMutation(ctx, mutation)
}
