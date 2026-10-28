package postgres

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

var _ assistant.EvidenceAuthority = (*Store)(nil)

func (store *Store) AuthorizeAssistantEvidence(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	return store.authorizeAssistant(ctx, actor, run, at, false)
}

func (store *Store) AuthorizeAssistantAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	return store.authorizeAssistant(ctx, actor, run, at, true)
}

func (store *Store) authorizeAssistant(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time, admission bool) error {
	if ctx == nil || ctx.Err() != nil || !actor.Valid() || !assistant.Timestamp(at) || !assistantEvidenceShape(run) {
		return assistant.ErrInvalid
	}
	if admission != (run.Version == 0) {
		return assistant.ErrInvalid
	}
	if actor.UserID != run.OwnerID {
		return assistant.ErrDenied
	}
	return store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		now, err := assistantNow(ctx, tx)
		if err != nil {
			return err
		}
		if admission {
			// This internal shape is checked before a Submit transaction exists.
			// Bind current configuration rather than trusting a supplied version.
			run.ConfigurationVersion = control.Version
			if !run.Deadline.After(now) || run.CreatedAt.After(now) {
				return assistant.ErrDenied
			}
			return assistantAuthority(ctx, tx, control, assistantRecord{Run: run, Actor: actor}, now)
		}
		stored, err := assistantRead(ctx, tx, run.ID)
		if err != nil {
			return err
		}
		if stored.Actor != actor || !assistantEvidenceSameRun(stored.Run, run) {
			return assistant.ErrDenied
		}
		if stored.State != assistant.Queued && stored.State != assistant.Running && stored.State != assistant.Completed {
			return assistant.ErrDenied
		}
		if stored.State != assistant.Completed && !stored.Deadline.After(now) || stored.State == assistant.Running && !stored.LeaseUntil.After(now) {
			return assistant.ErrDenied
		}
		return assistantAuthority(ctx, tx, control, stored, now)
	})
}

func assistantEvidenceShape(run assistant.Run) bool {
	if !assistant.Identifier(run.ID) || !assistant.Identifier(run.ConversationID) || !assistant.Identifier(run.OwnerID) || !assistant.Identifier(run.SourceID) || !run.Goal.Valid() || run.ResourceID != "" && !assistant.Identifier(run.ResourceID) || run.Goal == assistant.ExplainResource && run.ResourceID == "" || run.SourceVersion < 1 || run.RestoreGeneration < 0 || run.PolicyVersion < 1 || !assistant.Identifier(run.ProfileID) || run.ProfileVersion < 1 {
		return false
	}
	if !assistant.Timestamp(run.From) || !assistant.Timestamp(run.Until) || run.Until.Before(run.From) || run.Until.Sub(run.From) > 24*time.Hour || !assistant.Timestamp(run.CreatedAt) || !assistant.Timestamp(run.UpdatedAt) || run.UpdatedAt.Before(run.CreatedAt) || run.Until.After(run.CreatedAt) || !assistant.Timestamp(run.Deadline) || !run.Deadline.After(run.CreatedAt) || run.Deadline.Sub(run.CreatedAt) > assistant.MaximumRunDuration {
		return false
	}
	if run.ReservedUnits < 1 || run.ReservedUnits > assistant.MaximumReservationUnits || run.UsedUnits < 0 || run.UsedUnits > run.ReservedUnits || run.ToolCalls < 0 || run.ToolCalls > assistant.MaximumToolCalls || run.InferenceCalls < 0 || run.InferenceCalls > assistant.MaximumInferenceCalls || run.Attempts < 0 || run.Attempts > assistant.MaximumClaims || run.Fence < 0 || run.Version < 0 || run.Version > assistant.MaximumStatusVersions {
		return false
	}
	if run.Version == 0 {
		return run.ConfigurationVersion == 0 && run.State == assistant.Queued && run.Volatile && run.Fence == 0 && run.Attempts == 0 && run.ToolCalls == 0 && run.InferenceCalls == 0 && run.UsedUnits == 0 && !run.UsageUncertain && !run.UsageConservative && run.LeaseOwner == "" && run.LeaseUntil.IsZero() && run.UpdatedAt.Equal(run.CreatedAt)
	}
	if run.ConfigurationVersion < 1 || run.State != assistant.Queued && run.State != assistant.Running && !run.State.Terminal() {
		return false
	}
	if run.State == assistant.Running {
		return run.Fence > 0 && run.Attempts > 0 && assistant.Identifier(run.LeaseOwner) && assistant.Timestamp(run.LeaseUntil) && !run.LeaseUntil.After(run.Deadline)
	}
	return run.LeaseOwner == "" && run.LeaseUntil.IsZero()
}

func assistantEvidenceSameRun(current, observed assistant.Run) bool {
	for _, run := range []*assistant.Run{&current, &observed} {
		run.From, run.Until = run.From.UTC(), run.Until.UTC()
		run.CreatedAt, run.UpdatedAt, run.Deadline = run.CreatedAt.UTC(), run.UpdatedAt.UTC(), run.Deadline.UTC()
		run.LeaseUntil = run.LeaseUntil.UTC()
	}
	if current.State == assistant.Running && observed.State == assistant.Running {
		// Read authority survives renewal and forward step progress only inside
		// one live claim. This is never the compare-and-swap for a write.
		if current.Version < observed.Version || current.UpdatedAt.Before(observed.UpdatedAt) || current.LeaseUntil.Before(observed.LeaseUntil) || current.ToolCalls < observed.ToolCalls || current.InferenceCalls < observed.InferenceCalls || current.UsedUnits < observed.UsedUnits {
			return false
		}
		if current.Version == observed.Version && (!current.UpdatedAt.Equal(observed.UpdatedAt) || current.ToolCalls != observed.ToolCalls || current.InferenceCalls != observed.InferenceCalls || current.UsedUnits != observed.UsedUnits) {
			return false
		}
		observed.Version, observed.UpdatedAt, observed.LeaseUntil = current.Version, current.UpdatedAt, current.LeaseUntil
		observed.ToolCalls, observed.InferenceCalls, observed.UsedUnits = current.ToolCalls, current.InferenceCalls, current.UsedUnits
	}
	return current == observed
}
