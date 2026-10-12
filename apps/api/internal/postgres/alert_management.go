package postgres

import (
	"context"
	"errors"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const alertForAcknowledgementStatement = `SELECT id, rule_id, source_id, environment_id, resource_id, state, severity,
       evidence_start_at_us, evidence_end_at_us, started_at_us, updated_at_us,
       acknowledged_at_us, resolved_at_us, summary_code, synthetic
FROM alerts
WHERE id = $1
FOR UPDATE`

var _ alerting.ManagementRepository = (*Store)(nil)

func (store *Store) AcknowledgeAlert(ctx context.Context, command domain.AlertAcknowledgement) (domain.Alert, error) {
	if store == nil || ctx == nil || ctx.Err() != nil || command.Validate() != nil {
		return domain.Alert{}, domain.ErrRepositoryInvalidInput
	}
	var acknowledged domain.Alert
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		current, err := scanAlert(transaction.transaction.QueryRow(ctx, alertForAcknowledgementStatement, string(command.AlertID)))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRepositoryNotFound
		}
		if err != nil {
			return domain.ErrRepositoryUnavailable
		}
		if current.State != domain.AlertFiring {
			return domain.ErrRepositoryConflict
		}
		var dedupeKey string
		if err := transaction.transaction.QueryRow(ctx, `SELECT dedupe_key FROM alerts WHERE id = $1`, string(command.AlertID)).Scan(&dedupeKey); err != nil {
			return domain.ErrRepositoryUnavailable
		}
		acknowledged, err = alerting.Acknowledge(current, command.OccurredAt)
		if err != nil {
			return domain.ErrRepositoryConflict
		}
		previous := current.State
		userID := command.UserID
		mutation := domain.AlertMutation{
			Alert: acknowledged, DedupeKey: dedupeKey, ExpectedUpdatedAt: &current.UpdatedAt,
			ExpectedState: &previous, AcknowledgedByUserID: &userID,
			Transition: &domain.AlertTransition{
				ID: command.TransitionID, AlertID: acknowledged.ID, From: &previous, To: acknowledged.State,
				OccurredAt: command.OccurredAt,
				Actor:      domain.AuditActor{Kind: domain.AuditActorUser, ID: string(command.UserID)},
				ReasonCode: "alert.acknowledged", RequestID: &command.Audit.RequestID,
				EvidenceRange: acknowledged.EvidenceRange,
			},
		}
		repositories := &transactionRepositories{transaction: transaction}
		if err := repositories.SaveAlertMutation(ctx, mutation); err != nil {
			return err
		}
		return repositories.AppendAuditEntries(ctx, []domain.AuditEntry{command.Audit})
	})
	if err != nil {
		if errors.Is(err, domain.ErrRepositoryNotFound) || errors.Is(err, domain.ErrRepositoryConflict) || errors.Is(err, domain.ErrRepositoryInvalidInput) {
			return domain.Alert{}, err
		}
		return domain.Alert{}, errors.Join(domain.ErrRepositoryUnavailable, err)
	}
	return acknowledged, nil
}
