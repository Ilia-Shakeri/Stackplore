package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"golang.org/x/crypto/argon2"
)

func TestAuthRepositoryInitialSetupRaceCommitsOneCompleteState(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	issuedAt := authRepositoryTestTime()
	commits := make([]auth.InitialSetupCommit, 2)
	for index := range commits {
		suffix := fmt.Sprintf("race_%d", index)
		preauth, use := authRepositoryTestPreauth(t, store, suffix, auth.PreauthPurposeSetup, issuedAt, issuedAt.Add(time.Second))
		if err := store.InsertPreauth(ctx, preauth); err != nil {
			t.Fatal(err)
		}
		user := authRepositoryTestUser("usr_"+suffix, "admin"+fmt.Sprint(index), auth.RoleAdministrator, issuedAt.Add(time.Second))
		commits[index] = auth.InitialSetupCommit{
			Preauth:       use,
			ClearThrottle: authRepositoryTestThrottle(auth.ThrottleScopeSetupSource, byte(index+1)),
			User:          user,
			Credential:    authRepositoryTestCredential(byte(index + 1)),
			Session:       authRepositoryTestSession(t, store, user, "sess_"+suffix, suffix, issuedAt.Add(time.Second)),
			Audit: authRepositoryTestAudit(
				"audit_setup_"+suffix,
				domain.AuditActor{Kind: domain.AuditActorSystem},
				"auth.setup",
				user.ID,
				"auth.setup_succeeded",
				issuedAt.Add(time.Second),
			),
		}
	}

	start := make(chan struct{})
	results := make(chan error, len(commits))
	var wait sync.WaitGroup
	for _, commit := range commits {
		commit := commit
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- store.CompleteInitialSetup(ctx, commit)
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, auth.ErrAuthRepositoryConflict):
			conflicts++
		default:
			t.Fatalf("setup race error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("setup race successes = %d, conflicts = %d", successes, conflicts)
	}
	assertAuthRepositoryCount(t, store, "users", 1)
	assertAuthRepositoryCount(t, store, "sessions", 1)
	assertAuthRepositoryCount(t, store, "audit_entries", 1)
	var completed int64
	var closed int64
	if err := store.db.QueryRow(`SELECT setup_completed_at_us, network_setup_closed_at_us
FROM installation_metadata WHERE singleton_id = 1`).Scan(&completed, &closed); err != nil || completed != closed {
		t.Fatalf("setup marks = %d/%d, error = %v", completed, closed, err)
	}
	required, err := store.SetupRequired(ctx)
	if err != nil || required {
		t.Fatalf("SetupRequired() = %v, %v", required, err)
	}
}

func TestAuthRepositorySetupFailureConsumesProofAtomically(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	issuedAt := authRepositoryTestTime()
	preauth, use := authRepositoryTestPreauth(t, store, "setup_fail", auth.PreauthPurposeSetup, issuedAt, issuedAt.Add(time.Second))
	if err := store.InsertPreauth(ctx, preauth); err != nil {
		t.Fatal(err)
	}
	key := authRepositoryTestThrottle(auth.ThrottleScopeSetupSource, 7)
	commit := auth.SetupFailureCommit{
		Preauth: use,
		Throttle: auth.ThrottleFailure{
			Key: key, OccurredAt: use.ConsumedAt, Window: time.Minute,
			MaximumFailures: 1, BlockDuration: time.Minute, Retention: time.Minute,
		},
		Audit: authRepositoryTestAudit(
			"audit_setup_fail",
			domain.AuditActor{Kind: domain.AuditActorSystem},
			"auth.setup",
			"",
			"auth.setup_denied",
			use.ConsumedAt,
		),
	}
	commit.Audit.Outcome = domain.AuditDenied
	if err := store.CommitSetupFailure(ctx, commit); err != nil {
		t.Fatal(err)
	}
	if err := store.ConsumePreauth(ctx, use); !errors.Is(err, auth.ErrAuthRepositoryRejected) {
		t.Fatalf("ConsumePreauth(reuse) error = %v", err)
	}
	state, err := store.CheckThrottle(ctx, key, use.ConsumedAt)
	if err != nil || !state.BlockedUntil.Equal(use.ConsumedAt.Add(time.Minute)) {
		t.Fatalf("throttle = %+v, error = %v", state, err)
	}

	secondIssuedAt := use.ConsumedAt.Add(2 * time.Minute)
	secondPreauth, secondUse := authRepositoryTestPreauth(t, store, "setup_fail_second", auth.PreauthPurposeSetup, secondIssuedAt, secondIssuedAt.Add(time.Second))
	if err := store.InsertPreauth(ctx, secondPreauth); err != nil {
		t.Fatal(err)
	}
	secondCommit := commit
	secondCommit.Preauth = secondUse
	secondCommit.Throttle.OccurredAt = secondUse.ConsumedAt
	secondCommit.Audit = authRepositoryTestAudit(
		"audit_setup_fail_second",
		domain.AuditActor{Kind: domain.AuditActorSystem},
		"auth.setup",
		"",
		"auth.setup_denied",
		secondUse.ConsumedAt,
	)
	secondCommit.Audit.Outcome = domain.AuditDenied
	if err := store.CommitSetupFailure(ctx, secondCommit); err != nil {
		t.Fatalf("CommitSetupFailure(after expiry) error = %v", err)
	}
	secondState, err := store.CheckThrottle(ctx, key, secondUse.ConsumedAt)
	if err != nil || !secondState.BlockedUntil.Equal(secondUse.ConsumedAt.Add(time.Minute)) {
		t.Fatalf("second throttle = %+v, error = %v", secondState, err)
	}
	assertAuthRepositoryCount(t, store, "audit_entries", 2)
}

func TestAuthRepositoryPersistsRecentPasswordThrottleWithAudit(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	createdAt := authRepositoryTestTime()
	administrator := authRepositoryTestUser("usr_recent_throttle", "recentadmin", auth.RoleAdministrator, createdAt)
	session := authRepositoryTestSession(t, store, administrator, "sess_recent_throttle", "recent_throttle", createdAt)
	seedAuthRepositoryUsers(t, store, []auth.User{administrator}, []auth.Session{session})
	actor := auth.Principal{
		UserID: administrator.ID, SessionID: session.ID, Role: administrator.Role,
		AuthRevision: administrator.AuthRevision,
	}
	occurredAt := createdAt.Add(time.Second)
	key := authRepositoryTestThrottle(auth.ThrottleScopeRecentUser, 19)
	audit := authRepositoryTestAudit(
		"audit_recent_throttle",
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)},
		"auth.reauthenticate",
		actor.UserID,
		"auth.reauthentication_denied",
		occurredAt,
	)
	audit.Outcome = domain.AuditDenied
	failure := auth.ThrottleFailure{
		Key: key, OccurredAt: occurredAt, Window: time.Minute,
		MaximumFailures: 1, BlockDuration: 2 * time.Minute, Retention: 3 * time.Minute,
	}
	if err := store.CommitRecentAuthFailure(ctx, auth.RecentAuthFailureCommit{
		Actor: actor, Throttle: &failure, Audit: audit,
	}); err != nil {
		t.Fatal(err)
	}
	state, err := store.CheckThrottle(ctx, key, occurredAt)
	if err != nil || !state.BlockedUntil.Equal(occurredAt.Add(2*time.Minute)) {
		t.Fatalf("recent throttle = %+v, error = %v", state, err)
	}
	var audits int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM audit_entries WHERE id = ?`, audit.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("recent throttle audits = %d, error = %v", audits, err)
	}
}

func TestAuthRepositoryPreauthPoolHasHardBoundAndPrunesConsumedRows(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	now := authRepositoryTestTime()
	_, err := store.db.ExecContext(ctx, `WITH RECURSIVE sequence(value) AS (
    VALUES (1)
    UNION ALL
    SELECT value + 1 FROM sequence WHERE value < ?
)
INSERT INTO preauth_challenges
(id, purpose, cookie_digest, header_digest, digest_key_id, created_at_us, expires_at_us)
SELECT printf('pre_bulk_%d', value), 'login', randomblob(32), randomblob(32), 'key_bulk', ?, ?
FROM sequence`, maximumActivePreauth, now.Add(-time.Second).UnixMicro(), now.Add(time.Minute).UnixMicro())
	if err != nil {
		t.Fatal(err)
	}
	preauth, _ := authRepositoryTestPreauth(t, store, "over_cap", auth.PreauthPurposeLogin, now, now.Add(time.Second))
	if err := store.InsertPreauth(ctx, preauth); !errors.Is(err, auth.ErrAuthRepositoryRejected) {
		t.Fatalf("InsertPreauth(over cap) error = %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE preauth_challenges
SET consumed_at_us = created_at_us WHERE id = 'pre_bulk_1'`); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertPreauth(ctx, preauth); err != nil {
		t.Fatalf("InsertPreauth(after prune) error = %v", err)
	}
	assertAuthRepositoryCount(t, store, "preauth_challenges", maximumActivePreauth)
}

func TestAuthRepositoryConcurrentAdminDemotionsLeaveOneAdministrator(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	createdAt := authRepositoryTestTime()
	adminOne := authRepositoryTestUser("usr_admin_one", "adminone", auth.RoleAdministrator, createdAt)
	adminTwo := authRepositoryTestUser("usr_admin_two", "admintwo", auth.RoleAdministrator, createdAt)
	sessionOne := authRepositoryTestSession(t, store, adminOne, "sess_admin_one", "admin_one", createdAt)
	sessionTwo := authRepositoryTestSession(t, store, adminTwo, "sess_admin_two", "admin_two", createdAt)
	seedAuthRepositoryUsers(t, store, []auth.User{adminOne, adminTwo}, []auth.Session{sessionOne, sessionTwo})
	actor := auth.Principal{UserID: adminOne.ID, SessionID: sessionOne.ID, Role: adminOne.Role, AuthRevision: adminOne.AuthRevision}
	grantTime := createdAt.Add(time.Second)
	operationTime := createdAt.Add(2 * time.Second)
	grantOne := issueAuthRepositoryGrant(t, store, actor, "demote_one", auth.RecentAuthPurposeUserUpdate, grantTime, operationTime)
	grantTwo := issueAuthRepositoryGrant(t, store, actor, "demote_two", auth.RecentAuthPurposeUserUpdate, grantTime, operationTime)

	nextOne := adminOne
	nextOne.Role = auth.RoleViewer
	nextOne.AuthRevision++
	nextOne.UpdatedAt = operationTime
	nextOne.Version++
	nextTwo := adminTwo
	nextTwo.Role = auth.RoleViewer
	nextTwo.AuthRevision++
	nextTwo.UpdatedAt = operationTime
	nextTwo.Version++
	commits := []auth.UserUpdateCommit{
		{
			Actor: actor, Grant: grantOne, User: nextOne, ExpectedVersion: adminOne.Version,
			Audit: authRepositoryTestAudit("audit_demote_one", domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, "auth.user_update", adminOne.ID, "auth.user_updated", operationTime),
		},
		{
			Actor: actor, Grant: grantTwo, User: nextTwo, ExpectedVersion: adminTwo.Version,
			Audit: authRepositoryTestAudit("audit_demote_two", domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, "auth.user_update", adminTwo.ID, "auth.user_updated", operationTime),
		},
	}
	results := make(chan error, 2)
	start := make(chan struct{})
	for _, commit := range commits {
		commit := commit
		go func() {
			<-start
			results <- store.CommitUserUpdate(ctx, commit)
		}()
	}
	close(start)
	first := <-results
	second := <-results
	successes := 0
	for _, err := range []error{first, second} {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, auth.ErrAuthRepositoryRejected) && !errors.Is(err, auth.ErrAuthRepositoryConflict) {
			t.Fatalf("demotion race error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("demotion race successes = %d, errors = %v / %v", successes, first, second)
	}
	var administrators int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM users
WHERE role = 'administrator' AND enabled = 1 AND deleted_at_us IS NULL`).Scan(&administrators); err != nil || administrators != 1 {
		t.Fatalf("enabled administrators = %d, error = %v", administrators, err)
	}
}

func TestAuthRepositoryFailedMutationRollsBackRecentGrant(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	createdAt := authRepositoryTestTime()
	administrator := authRepositoryTestUser("usr_admin", "adminuser", auth.RoleAdministrator, createdAt)
	viewer := authRepositoryTestUser("usr_viewer", "viewuser", auth.RoleViewer, createdAt)
	adminSession := authRepositoryTestSession(t, store, administrator, "sess_admin", "rollback_admin", createdAt)
	viewerSession := authRepositoryTestSession(t, store, viewer, "sess_viewer", "rollback_viewer", createdAt)
	seedAuthRepositoryUsers(t, store, []auth.User{administrator, viewer}, []auth.Session{adminSession, viewerSession})
	actor := auth.Principal{UserID: administrator.ID, SessionID: adminSession.ID, Role: administrator.Role, AuthRevision: administrator.AuthRevision}
	operationTime := createdAt.Add(2 * time.Second)
	grant := issueAuthRepositoryGrant(t, store, actor, "rollback", auth.RecentAuthPurposeUserUpdate, createdAt.Add(time.Second), operationTime)

	staleUser := viewer
	staleUser.DisplayName = "Changed Viewer"
	staleUser.UpdatedAt = operationTime
	staleUser.Version = 3
	stale := auth.UserUpdateCommit{
		Actor: actor, Grant: grant, User: staleUser, ExpectedVersion: 2,
		Audit: authRepositoryTestAudit("audit_rollback", domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, "auth.user_update", viewer.ID, "auth.user_updated", operationTime),
	}
	if err := store.CommitUserUpdate(ctx, stale); !errors.Is(err, auth.ErrAuthRepositoryConflict) {
		t.Fatalf("CommitUserUpdate(stale) error = %v", err)
	}

	updatedUser := viewer
	updatedUser.DisplayName = "Changed Viewer"
	updatedUser.UpdatedAt = operationTime
	updatedUser.Version = 2
	valid := stale
	valid.User = updatedUser
	valid.ExpectedVersion = 1
	if err := store.CommitUserUpdate(ctx, valid); err != nil {
		t.Fatalf("CommitUserUpdate(retry) error = %v", err)
	}
	if err := store.CommitUserUpdate(ctx, valid); !errors.Is(err, auth.ErrAuthRepositoryRejected) {
		t.Fatalf("CommitUserUpdate(grant reuse) error = %v", err)
	}
	var consumedAt int64
	if err := store.db.QueryRowContext(ctx, `SELECT consumed_at_us FROM recent_auth_grants
WHERE purpose = ? AND user_id = ?`, string(auth.RecentAuthPurposeUserUpdate), string(actor.UserID)).Scan(&consumedAt); err != nil || consumedAt != operationTime.UnixMicro() {
		t.Fatalf("grant consumed_at = %d, error = %v", consumedAt, err)
	}
	var auditCount int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM audit_entries WHERE id = 'audit_rollback'`).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("mutation audit count = %d, error = %v", auditCount, err)
	}
}

func TestAuthRepositoryConcurrentGrantUseCommitsOnce(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	createdAt := authRepositoryTestTime()
	administrator := authRepositoryTestUser("usr_grant_admin", "grantadmin", auth.RoleAdministrator, createdAt)
	viewer := authRepositoryTestUser("usr_grant_viewer", "grantviewer", auth.RoleViewer, createdAt)
	adminSession := authRepositoryTestSession(t, store, administrator, "sess_grant_admin", "grant_admin", createdAt)
	viewerSession := authRepositoryTestSession(t, store, viewer, "sess_grant_viewer", "grant_viewer", createdAt)
	seedAuthRepositoryUsers(t, store, []auth.User{administrator, viewer}, []auth.Session{adminSession, viewerSession})
	actor := auth.Principal{UserID: administrator.ID, SessionID: adminSession.ID, Role: administrator.Role, AuthRevision: administrator.AuthRevision}
	operationTime := createdAt.Add(2 * time.Second)
	grant := issueAuthRepositoryGrant(t, store, actor, "one_use", auth.RecentAuthPurposeUserUpdate, createdAt.Add(time.Second), operationTime)
	updated := viewer
	updated.DisplayName = "One Use Result"
	updated.UpdatedAt = operationTime
	updated.Version++
	commit := auth.UserUpdateCommit{
		Actor: actor, Grant: grant, User: updated, ExpectedVersion: viewer.Version,
		Audit: authRepositoryTestAudit("audit_one_use", domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, "user.update", viewer.ID, "user.update_succeeded", operationTime),
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- store.CommitUserUpdate(ctx, commit)
		}()
	}
	close(start)
	first := <-results
	second := <-results
	successes := 0
	rejections := 0
	for _, err := range []error{first, second} {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, auth.ErrAuthRepositoryRejected):
			rejections++
		default:
			t.Fatalf("grant race error = %v", err)
		}
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("grant race successes = %d, rejections = %d", successes, rejections)
	}
	var auditCount int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM audit_entries WHERE id = 'audit_one_use'`).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("one-use audit count = %d, error = %v", auditCount, err)
	}
	stored, err := store.GetUser(ctx, viewer.ID)
	if err != nil || stored.DisplayName != updated.DisplayName || stored.Version != updated.Version {
		t.Fatalf("one-use user = %+v, error = %v", stored, err)
	}
}

func TestAuthRepositoryOperatorRecoveryClosesFreshSetupAndRevokesExistingSessions(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	recoveredAt := authRepositoryTestTime()
	requested := authRepositoryTestUser("usr_recovery", "recovery", auth.RoleAdministrator, recoveredAt)
	commit := auth.OperatorRecoveryCommit{
		OperatorID:    "operator_local",
		RequestedUser: requested,
		Credential:    authRepositoryTestCredential(91),
		RecoveredAt:   recoveredAt,
		Audit: authRepositoryTestAudit(
			"audit_recovery_create",
			domain.AuditActor{Kind: domain.AuditActorOperator, ID: "operator_local"},
			"auth.operator_recovery",
			requested.ID,
			"auth.operator_recovery_succeeded",
			recoveredAt,
		),
	}
	created, err := store.CommitOperatorRecovery(ctx, commit)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != requested.ID || created.Role != auth.RoleAdministrator || !created.Enabled {
		t.Fatalf("created recovery user = %+v", created)
	}
	var initialCompleted int64
	var initialClosed int64
	if err := store.db.QueryRowContext(ctx, `SELECT setup_completed_at_us, network_setup_closed_at_us
FROM installation_metadata WHERE singleton_id = 1`).Scan(&initialCompleted, &initialClosed); err != nil || initialCompleted != recoveredAt.UnixMicro() || initialClosed != recoveredAt.UnixMicro() {
		t.Fatalf("initial recovery marks = %d/%d, error = %v", initialCompleted, initialClosed, err)
	}

	sessionTime := recoveredAt.Add(time.Second)
	session := authRepositoryTestSession(t, store, created, "sess_recovery", "recovery_session", sessionTime)
	if err := store.withinTransaction(ctx, func(tx *transaction) error {
		return insertSessionTx(ctx, tx.transaction, session)
	}); err != nil {
		t.Fatal(err)
	}
	secondTime := recoveredAt.Add(2 * time.Second)
	secondRequested := created
	secondRequested.DisplayName = "Recovered Admin"
	secondRequested.AuthRevision++
	secondRequested.UpdatedAt = secondTime
	secondRequested.Version++
	second := auth.OperatorRecoveryCommit{
		OperatorID:    "operator_local",
		RequestedUser: secondRequested,
		Credential:    authRepositoryTestCredential(92),
		RecoveredAt:   secondTime,
		Audit: authRepositoryTestAudit(
			"audit_recovery_update",
			domain.AuditActor{Kind: domain.AuditActorOperator, ID: "operator_local"},
			"auth.operator_recovery",
			secondRequested.ID,
			"auth.operator_recovery_succeeded",
			secondTime,
		),
	}
	updated, err := store.CommitOperatorRecovery(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.AuthRevision != 2 || updated.Version != 2 || updated.DisplayName != secondRequested.DisplayName {
		t.Fatalf("updated recovery user = %+v", updated)
	}
	var revokedAt int64
	if err := store.db.QueryRowContext(ctx, `SELECT revoked_at_us FROM sessions WHERE id = ?`, string(session.ID)).Scan(&revokedAt); err != nil || revokedAt != secondTime.UnixMicro() {
		t.Fatalf("recovery session revoke = %d, error = %v", revokedAt, err)
	}
	var finalCompleted int64
	var finalClosed int64
	if err := store.db.QueryRowContext(ctx, `SELECT setup_completed_at_us, network_setup_closed_at_us
FROM installation_metadata WHERE singleton_id = 1`).Scan(&finalCompleted, &finalClosed); err != nil || finalCompleted != initialCompleted || finalClosed != initialClosed {
		t.Fatalf("final recovery marks = %d/%d, error = %v", finalCompleted, finalClosed, err)
	}
}

func TestAuthRepositoryCursorIsCanonicalBoundAndTamperEvident(t *testing.T) {
	store := openAuthRepositoryTestStore(t)
	ctx := context.Background()
	createdAt := authRepositoryTestTime()
	users := []auth.User{
		authRepositoryTestUser("usr_cursor_a", "alpha", auth.RoleAdministrator, createdAt),
		authRepositoryTestUser("usr_cursor_b", "bravo", auth.RoleViewer, createdAt),
		authRepositoryTestUser("usr_cursor_c", "charlie", auth.RoleViewer, createdAt),
	}
	sessions := make([]auth.Session, len(users))
	for index := range users {
		sessions[index] = authRepositoryTestSession(t, store, users[index], "sess_cursor_"+fmt.Sprint(index), "cursor_"+fmt.Sprint(index), createdAt)
	}
	seedAuthRepositoryUsers(t, store, users, sessions)

	query := auth.UserListQuery{Limit: 1}
	first, err := store.ListUsers(ctx, query)
	if err != nil || len(first.Items) != 1 || first.Items[0].ID != users[0].ID || first.NextCursor == "" {
		t.Fatalf("first page = %+v; error = %v", first, err)
	}
	for _, character := range first.NextCursor {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		t.Fatalf("cursor has non-base64url character %q", character)
	}
	query.Cursor = first.NextCursor
	second, err := store.ListUsers(ctx, query)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != users[1].ID || second.NextCursor == "" {
		t.Fatalf("second page = %+v; error = %v", second, err)
	}

	tampered := query
	replacement := byte('A')
	if tampered.Cursor[0] == replacement {
		replacement = 'B'
	}
	tampered.Cursor = string(replacement) + tampered.Cursor[1:]
	if _, err := store.ListUsers(ctx, tampered); !errors.Is(err, auth.ErrAuthRepositoryRejected) {
		t.Fatalf("tampered cursor error = %v", err)
	}
	changedFilter := query
	changedFilter.Search = "bravo"
	if _, err := store.ListUsers(ctx, changedFilter); !errors.Is(err, auth.ErrAuthRepositoryRejected) {
		t.Fatalf("filter-bound cursor error = %v", err)
	}
}

func openAuthRepositoryTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), Options{
		DataDirectory: filepath.Join(t.TempDir(), "data"),
		BusyTimeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return store
}

func authRepositoryTestTime() time.Time {
	return time.Now().UTC().Add(time.Second).Truncate(time.Microsecond)
}

func authRepositoryTestCredential(seed byte) auth.PasswordCredential {
	return auth.PasswordCredential{
		Algorithm:        auth.PasswordAlgorithmArgon2id,
		AlgorithmVersion: argon2.Version,
		TimeCost:         1,
		MemoryKiB:        8 * 1024,
		Parallelism:      1,
		Salt:             bytes.Repeat([]byte{seed}, 16),
		Hash:             bytes.Repeat([]byte{seed + 1}, 16),
	}
}

func authRepositoryTestUser(id string, login string, role auth.Role, createdAt time.Time) auth.User {
	canonical, err := auth.CanonicalLoginName(login)
	if err != nil {
		panic(err)
	}
	return auth.User{
		ID: auth.UserID(id), LoginName: login, CanonicalLoginName: canonical,
		DisplayName: login + " display", Role: role, Enabled: true,
		AuthRevision: 1, CreatedAt: createdAt, UpdatedAt: createdAt, Version: 1,
	}
}

func authRepositoryTestPreauth(t *testing.T, store *Store, suffix string, purpose auth.PreauthPurpose, createdAt time.Time, consumedAt time.Time) (auth.Preauth, auth.PreauthUse) {
	t.Helper()
	keyID, key, err := store.AuthDigestKey()
	if err != nil {
		t.Fatal(err)
	}
	cookie := authRepositoryTestOpaque(t, "cookie_"+suffix)
	proof := authRepositoryTestOpaque(t, "proof_"+suffix)
	cookieDigest, err := auth.DigestPreauthCookie(key, cookie)
	if err != nil {
		t.Fatal(err)
	}
	proofDigest, err := auth.DigestPreauthProof(key, proof)
	if err != nil {
		t.Fatal(err)
	}
	preauth := auth.Preauth{
		ID: "pre_" + suffix, Purpose: purpose, CookieDigest: cookieDigest,
		ProofDigest: proofDigest, DigestKeyID: keyID, CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(5 * time.Minute),
	}
	use := auth.PreauthUse{
		Purpose: purpose, CookieDigest: cookieDigest, ProofDigest: proofDigest,
		DigestKeyID: keyID, ConsumedAt: consumedAt,
	}
	return preauth, use
}

func authRepositoryTestSession(t *testing.T, store *Store, user auth.User, id string, label string, createdAt time.Time) auth.Session {
	t.Helper()
	keyID, key, err := store.AuthDigestKey()
	if err != nil {
		t.Fatal(err)
	}
	tokenDigest, err := auth.DigestSessionToken(key, authRepositoryTestOpaque(t, "session_"+label))
	if err != nil {
		t.Fatal(err)
	}
	csrfDigest, err := auth.DigestCSRFProof(key, authRepositoryTestOpaque(t, "csrf_"+label))
	if err != nil {
		t.Fatal(err)
	}
	return auth.Session{
		ID: auth.SessionID(id), UserID: user.ID,
		TokenDigest: tokenDigest, TokenDigestKeyID: keyID,
		CSRFDigest: csrfDigest, CSRFDigestKeyID: keyID,
		UserAuthRevision: user.AuthRevision, CreatedAt: createdAt, LastSeenAt: createdAt,
		IdleExpiresAt: createdAt.Add(30 * time.Minute), AbsoluteExpiresAt: createdAt.Add(12 * time.Hour),
	}
}

func authRepositoryTestOpaque(t *testing.T, label string) auth.OpaqueToken {
	t.Helper()
	sum := sha256.Sum256([]byte(label))
	encoded := base64.RawURLEncoding.EncodeToString(sum[:])
	token, err := auth.ParseOpaqueToken(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func authRepositoryTestThrottle(scope auth.ThrottleScope, seed byte) auth.ThrottleKey {
	var digest [auth.DigestBytes]byte
	digest[0] = seed
	return auth.ThrottleKey{Scope: scope, Digest: digest}
}

func authRepositoryTestAudit(id string, actor domain.AuditActor, action string, target auth.UserID, reason string, occurredAt time.Time) domain.AuditEntry {
	return domain.AuditEntry{
		ID: domain.AuditEntryID(id), OccurredAt: occurredAt, Actor: actor,
		Action:  domain.AuditAction(action),
		Target:  domain.AuditTarget{Kind: domain.AuditTargetKind("auth.account"), ID: string(target)},
		Outcome: domain.AuditSuccess, RequestID: domain.RequestID("req_" + id),
		ReasonCode: domain.ReasonCode(reason),
	}
}

func seedAuthRepositoryUsers(t *testing.T, store *Store, users []auth.User, sessions []auth.Session) {
	t.Helper()
	if len(users) == 0 || len(users) != len(sessions) {
		t.Fatal("invalid auth seed")
	}
	ctx := context.Background()
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		result, err := tx.transaction.ExecContext(ctx, `UPDATE installation_metadata
SET setup_completed_at_us = ?, network_setup_closed_at_us = ?
WHERE singleton_id = 1 AND setup_completed_at_us IS NULL AND network_setup_closed_at_us IS NULL`,
			users[0].CreatedAt.UnixMicro(), users[0].CreatedAt.UnixMicro())
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return errors.New("setup metadata seed failed")
		}
		for index, user := range users {
			if err := insertUserTx(ctx, tx.transaction, user); err != nil {
				return err
			}
			if err := insertCredentialTx(ctx, tx.transaction, user.ID, authRepositoryTestCredential(byte(index+11))); err != nil {
				return err
			}
			if err := insertSessionTx(ctx, tx.transaction, sessions[index]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func issueAuthRepositoryGrant(t *testing.T, store *Store, actor auth.Principal, suffix string, purpose auth.RecentAuthPurpose, createdAt time.Time, consumedAt time.Time) auth.RecentGrantUse {
	t.Helper()
	keyID, key, err := store.AuthDigestKey()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := auth.DigestRecentAuthProof(key, authRepositoryTestOpaque(t, "grant_"+suffix))
	if err != nil {
		t.Fatal(err)
	}
	grant := auth.RecentGrant{
		ID: "grant_" + suffix, UserID: actor.UserID, SessionID: actor.SessionID,
		Purpose: purpose, Digest: digest, DigestKeyID: keyID,
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	}
	audit := authRepositoryTestAudit(
		"audit_grant_"+suffix,
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)},
		"auth.reauthenticate",
		actor.UserID,
		"auth.reauthentication_succeeded",
		createdAt,
	)
	if err := store.CommitRecentGrant(context.Background(), auth.RecentGrantCommit{
		Actor: actor, Grant: grant,
		ClearThrottle: authRepositoryTestThrottle(auth.ThrottleScopeRecentUser, byte(len(suffix))),
		Audit:         audit,
	}); err != nil {
		t.Fatal(err)
	}
	return auth.RecentGrantUse{
		Actor: actor, Purpose: purpose, Digest: digest, DigestKeyID: keyID, ConsumedAt: consumedAt,
	}
}

func assertAuthRepositoryCount(t *testing.T, store *Store, table string, wanted int) {
	t.Helper()
	allowed := map[string]bool{
		"users": true, "sessions": true, "audit_entries": true, "preauth_challenges": true,
	}
	if !allowed[table] {
		t.Fatalf("invalid count table %q", table)
	}
	var count int
	if err := store.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != wanted {
		t.Fatalf("%s count = %d, error = %v", table, count, err)
	}
}
