package postgres

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"github.com/jackc/pgx/v5"
	"time"
)

// The owning reader separately checks its admitted run and durable reservation.
// These ports derive document audience from the live database actor, not a caller role.
func (store *Store) ReadAssistantRunbook(ctx context.Context, actor assistant.Actor, reference runbook.Reference) (runbook.Content, error) {
	if ctx == nil || !actor.Valid() || reference.Validate() != nil {
		return runbook.Content{}, runbook.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var content runbook.Content
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		administrator, err := assistantRunbookActor(ctx, tx, actor)
		if err != nil {
			return err
		}
		content, err = readReviewedRunbookTx(ctx, tx, reference, administrator, true)
		if err != nil {
			return err
		}
		_, err = assistantRunbookActor(ctx, tx, actor)
		return err
	})
	if err != nil {
		return runbook.Content{}, assistantRunbookError(err)
	}
	return content, nil
}

// AuthorizeAssistantRunbook selects no document body, excerpt, title or source prose.
func (store *Store) AuthorizeAssistantRunbook(ctx context.Context, actor assistant.Actor, reference runbook.Reference) error {
	if ctx == nil || !actor.Valid() || reference.Validate() != nil {
		return runbook.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		administrator, err := assistantRunbookActor(ctx, tx, actor)
		if err != nil {
			return err
		}
		generation, _, err := runbookScope(ctx, tx, false)
		if err != nil {
			return err
		}
		if generation != reference.Generation {
			return runbook.ErrDenied
		}
		var marker int
		err = tx.QueryRow(ctx, `SELECT 1 FROM runbook_corpora c JOIN runbook_documents d ON d.corpus_id=c.id
WHERE c.id=$1 AND c.version=$2 AND c.digest=$3 AND c.restore_generation=$4 AND c.approved AND c.provenance_use_reviewed IS TRUE AND c.provenance_assistance_reviewed
AND (c.audience='all_users' OR $5::boolean) AND d.id=$6 AND d.locale=$7`, reference.CorpusID, reference.Version, reference.Digest[:], generation, administrator, reference.DocumentID, reference.Locale).Scan(&marker)
		if errors.Is(err, pgx.ErrNoRows) {
			return runbook.ErrDenied
		}
		if err != nil {
			return err
		}
		_, err = assistantRunbookActor(ctx, tx, actor)
		return err
	})
	return assistantRunbookError(err)
}

func assistantRunbookActor(ctx context.Context, tx pgx.Tx, actor assistant.Actor) (bool, error) {
	at, err := assistantNow(ctx, tx)
	if err != nil {
		return false, err
	}
	if err := assistantActor(ctx, tx, actor, at); err != nil {
		return false, err
	}
	var administrator bool
	err = tx.QueryRow(ctx, `SELECT role='administrator' FROM users WHERE id=$1`, actor.UserID).Scan(&administrator)
	return administrator, err
}
func assistantRunbookError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, assistant.ErrDenied) {
		return runbook.ErrDenied
	}
	return runbookError(err)
}
