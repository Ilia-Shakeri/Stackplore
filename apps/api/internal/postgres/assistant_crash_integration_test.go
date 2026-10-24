package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

const assistantCrashChildKey = "STACKPLORE_TEST_ASSISTANT_CRASH_CHILD"
const assistantCrashDispatchMarker = "synthetic_dispatch_1\n"

type assistantCrashCheckpoint struct {
	Phase   string `json:"phase"`
	RunID   string `json:"run_id"`
	Version int64  `json:"version"`
	Fence   int64  `json:"fence"`
	Step    int    `json:"step"`
}

func TestAssistantNativeCrashBoundaries(t *testing.T) {
	if os.Getenv(integrationDatabaseURLKey) == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	for _, phase := range []string{"admitted", "before_dispatch", "completion_before_commit"} {
		t.Run(phase, func(t *testing.T) { testAssistantNativeCrashBoundary(t, phase) })
	}
}

func testAssistantNativeCrashBoundary(t *testing.T, phase string) {
	store, actor, at := assistantFixture(t)
	options := validTestOptions()
	options.ConnectionString = os.Getenv(integrationDatabaseURLKey)
	options.Schema = store.pool.Config().ConnConfig.RuntimeParams["search_path"]
	if !assistantCrashSchema(options.Schema) {
		t.Fatal("crash_parent_schema_invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	observerConfig, err := pgx.ParseConfig(options.ConnectionString)
	if err != nil {
		t.Fatal("crash_parent_connection_invalid")
	}
	observerConfig.RuntimeParams["search_path"] = options.Schema
	observer, err := pgx.ConnectConfig(ctx, observerConfig)
	if err != nil {
		t.Fatal("crash_parent_observer_unavailable")
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if observer.Close(cleanup) != nil {
			t.Error("crash_parent_observer_cleanup_failed")
		}
	}()
	barrier := assistantCrashBarrier(options.Schema)
	if phase == "completion_before_commit" {
		if _, err := observer.Exec(ctx, `SELECT pg_advisory_lock($1)`, barrier); err != nil {
			t.Fatal("crash_parent_barrier_unavailable")
		}
	}
	directory := t.TempDir()
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal("crash_parent_nonce_unavailable")
	}
	nonce := hex.EncodeToString(random[:])
	if assistantCrashWrite(directory, "owner", []byte(nonce+"\n"+options.Schema+"\n")) != nil {
		t.Fatal("crash_parent_owner_unavailable")
	}
	if store.Close() != nil {
		t.Fatal("crash_parent_ownership_release_failed")
	}
	executable, err := os.Executable()
	if err != nil || !filepath.IsAbs(executable) {
		t.Fatal("crash_parent_executable_invalid")
	}
	command := exec.CommandContext(ctx, executable, "-test.run=^TestAssistantCrashChildProcess$", "-test.count=1")
	command.Env = assistantCrashEnvironment(options.ConnectionString, options.Schema, directory, nonce, phase, at)
	command.Stdin, command.Stdout, command.Stderr = nil, io.Discard, io.Discard
	command.WaitDelay = 2 * time.Second
	if command.Start() != nil {
		t.Fatal("crash_parent_child_start_failed")
	}
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	reaped := false
	defer func() {
		if reaped {
			return
		}
		_ = command.Process.Kill()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("crash_parent_child_cleanup_failed")
		}
	}()
	checkpoint := assistantCrashWaitCheckpoint(t, ctx, directory, phase, finished, &reaped)
	if phase == "completion_before_commit" {
		wait, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			var blocked bool
			err := observer.QueryRow(wait, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory'
AND classid=($1::bigint>>32)::oid AND objid=($1::bigint & 4294967295)::oid AND objsubid=1 AND NOT granted)`, barrier).Scan(&blocked)
			if err != nil {
				t.Fatal("crash_parent_commit_barrier_unavailable")
			}
			if blocked {
				break
			}
			select {
			case <-wait.Done():
				t.Fatal("crash_parent_commit_not_blocked")
			case <-ticker.C:
			}
		}
	}
	var pending, held int
	if err := observer.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_steps WHERE state='running'),
(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation')`).Scan(&pending, &held); err != nil || held != 100 || pending != checkpoint.Step {
		t.Fatal("crash_parent_reservation_not_committed")
	}
	marker, markerErr := assistantCrashRead(directory, "dispatch")
	expectDispatch := phase == "completion_before_commit"
	if expectDispatch && (markerErr != nil || string(marker) != assistantCrashDispatchMarker) || !expectDispatch && !errors.Is(markerErr, os.ErrNotExist) {
		t.Fatal("crash_parent_dispatch_checkpoint_invalid")
	}
	if command.Process.Kill() != nil {
		t.Fatal("crash_parent_exact_child_kill_failed")
	}
	select {
	case waitErr := <-finished:
		reaped = true
		if waitErr == nil || command.ProcessState == nil || command.ProcessState.Success() {
			t.Fatal("crash_parent_child_did_not_crash")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("crash_parent_child_wait_failed")
	}
	// The observer is still holding this schema's private barrier. A killed
	// completion transaction must have rolled back before recovery can proceed.
	if expectDispatch {
		var stepState string
		var used int64
		if err := observer.QueryRow(ctx, `SELECT step.state,run.used_units FROM assistant_steps AS step JOIN assistant_runs AS run ON run.id=step.run_id
WHERE run.id=$1 AND step.step_number=$2`, checkpoint.RunID, checkpoint.Step).Scan(&stepState, &used); err != nil || stepState != "running" || used != 0 {
			t.Fatal("crash_parent_uncommitted_completion_survived")
		}
		if _, err := observer.Exec(ctx, `DROP TRIGGER assistant_crash_pause ON assistant_status_events; DROP FUNCTION assistant_crash_pause()`); err != nil {
			t.Fatal("crash_parent_barrier_fixture_cleanup_failed")
		}
	}
	reopened, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("crash_parent_reopen_failed")
	}
	defer func() {
		if reopened.Close() != nil {
			t.Error("crash_parent_reopened_cleanup_failed")
		}
	}()
	expectedState, expectedHeld := assistant.Interrupted, int64(0)
	if phase != "admitted" {
		expectedState, expectedHeld = assistant.Uncertain, 100
	}
	var current assistant.Run
	for {
		now := time.Now().UTC().Truncate(time.Microsecond)
		if _, err := reopened.RecoverAssistant(ctx, now, 18); err != nil {
			t.Fatal("crash_parent_recovery_failed")
		}
		if _, found, err := reopened.ClaimAssistant(ctx, assistant.Claim{Worker: "crash_recovered_worker", ProcessID: "crash_recovered_process", At: now, Lease: assistant.MinimumLeaseDuration}); err != nil || found {
			t.Fatal("crash_parent_lost_work_was_dispatched_again")
		}
		current, err = reopened.GetAssistant(ctx, actor, checkpoint.RunID, now)
		if err != nil {
			t.Fatal("crash_parent_recovered_status_unavailable")
		}
		if current.State == expectedState {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("crash_parent_real_lease_expiry_not_recovered")
		case <-time.After(20 * time.Millisecond):
		}
	}
	var ledgerState string
	var reserved, used int64
	if err := reopened.pool.QueryRow(ctx, `SELECT ledger.state,account.reserved_units,account.used_units FROM assistant_usage_ledger AS ledger
CROSS JOIN assistant_usage_accounts AS account WHERE ledger.reservation_id=$1 AND account.account_key='installation'`, checkpoint.RunID).Scan(&ledgerState, &reserved, &used); err != nil || reserved != expectedHeld || used != 0 || (expectedHeld == 0 && ledgerState != "settled") || (expectedHeld != 0 && ledgerState != "unknown") {
		t.Fatal("crash_parent_usage_accounting_changed")
	}
	admission := assistantCrashAdmission(actor, phase, at)
	replayed, err := reopened.AdmitAssistant(ctx, admission)
	if err != nil || replayed.ID != current.ID || replayed.State != expectedState || replayed.Version != current.Version {
		t.Fatal("crash_parent_idempotent_replay_created_work")
	}
	admission.Fingerprint = sha256.Sum256([]byte("synthetic_changed_crash_input"))
	if _, err := reopened.AdmitAssistant(ctx, admission); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("crash_parent_changed_material_replayed")
	}
	if checkpoint.Step != 0 {
		old := assistant.Lease{RunID: checkpoint.RunID, Worker: "crash_original_worker", Version: checkpoint.Version, Fence: checkpoint.Fence, At: time.Now().UTC().Truncate(time.Microsecond)}
		if _, err := reopened.FinishAssistantStep(ctx, assistant.StepFinish{Lease: old, Number: checkpoint.Step, Units: 10}); !errors.Is(err, assistant.ErrConflict) {
			t.Fatal("crash_parent_old_completion_was_accepted")
		}
	}
	var runs, steps int
	if err := reopened.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs),(SELECT count(*) FROM assistant_steps WHERE kind='inference')`).Scan(&runs, &steps); err != nil || runs != 1 || steps != checkpoint.Step {
		t.Fatal("crash_parent_duplicate_metadata_accepted")
	}
	afterMarker, afterErr := assistantCrashRead(directory, "dispatch")
	if expectDispatch && (afterErr != nil || string(afterMarker) != string(marker)) || !expectDispatch && !errors.Is(afterErr, os.ErrNotExist) {
		t.Fatal("crash_parent_synthetic_dispatch_repeated")
	}
	t.Logf("case=%s owned_child_killed=true child_waited=true recovered=%s held_units=%d simulated_dispatch=%t engine_calls=0", phase, current.State, reserved, expectDispatch)
}

func TestAssistantCrashChildProcess(t *testing.T) {
	if os.Getenv(assistantCrashChildKey) != "v1" {
		t.Skip("native crash helper is not selected")
	}
	directory, schema := os.Getenv("STACKPLORE_TEST_ASSISTANT_CRASH_DIRECTORY"), os.Getenv("STACKPLORE_TEST_ASSISTANT_CRASH_SCHEMA")
	nonce, phase := os.Getenv("STACKPLORE_TEST_ASSISTANT_CRASH_NONCE"), os.Getenv("STACKPLORE_TEST_ASSISTANT_CRASH_PHASE")
	if !filepath.IsAbs(directory) || !assistantCrashSchema(schema) || len(nonce) != 64 || strings.ToLower(nonce) != nonce || (phase != "admitted" && phase != "before_dispatch" && phase != "completion_before_commit") {
		t.Fatal("crash_child_scope_invalid")
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("crash_child_directory_invalid")
	}
	if _, err := hex.DecodeString(nonce); err != nil {
		t.Fatal("crash_child_nonce_invalid")
	}
	owner, err := assistantCrashRead(directory, "owner")
	if err != nil || string(owner) != nonce+"\n"+schema+"\n" {
		t.Fatal("crash_child_owner_invalid")
	}
	atUS, err := strconv.ParseInt(os.Getenv("STACKPLORE_TEST_ASSISTANT_CRASH_AT"), 10, 64)
	if err != nil || atUS <= 0 || os.Getenv(integrationDatabaseURLKey) == "" {
		t.Fatal("crash_child_configuration_invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	options := validTestOptions()
	options.ConnectionString, options.Schema = os.Getenv(integrationDatabaseURLKey), schema
	// The parent owns the fixture gate. Taking it again here would deadlock.
	store, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("crash_child_storage_unavailable")
	}
	defer store.Close()
	actor := assistant.Actor{UserID: "usr_source_admin", AuthRevision: 1}
	var digest []byte
	if err := store.pool.QueryRow(ctx, `SELECT token_digest,token_digest_key_id FROM sessions WHERE user_id=$1`, actor.UserID).Scan(&digest, &actor.DigestKeyID); err != nil || len(digest) != len(actor.SessionDigest) {
		t.Fatal("crash_child_actor_unavailable")
	}
	copy(actor.SessionDigest[:], digest)
	clear(digest)
	question := []byte("Synthetic volatile working text.")
	defer func() { clear(question) }()
	defer runtime.KeepAlive(question)
	run, err := store.AdmitAssistant(ctx, assistantCrashAdmission(actor, phase, time.UnixMicro(atUS).UTC()))
	if err != nil {
		t.Fatal("crash_child_admission_failed")
	}
	stepNumber := 0
	if phase != "admitted" {
		claimed, found, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "crash_original_worker", ProcessID: "crash_original_process", At: time.Now().UTC().Truncate(time.Microsecond), Lease: assistant.MaximumLeaseDuration})
		if err != nil || !found || claimed.ID != run.ID {
			t.Fatal("crash_child_claim_failed")
		}
		var step assistant.Step
		run, step, err = store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(claimed, time.Now().UTC().Truncate(time.Microsecond)), Kind: assistant.Inference, MaximumUnits: 100})
		if err != nil {
			t.Fatal("crash_child_reservation_failed")
		}
		stepNumber = step.Number
		// A separate transaction observes the durable reservation and acquires
		// the control lock before the synthetic dispatch marker can be written.
		proof, err := store.pool.Begin(ctx)
		if err != nil {
			t.Fatal("crash_child_reservation_proof_unavailable")
		}
		var held, maximum int64
		if _, err := proof.Exec(ctx, `SELECT row_version FROM assistant_control WHERE singleton_id=1 FOR UPDATE`); err != nil {
			_ = proof.Rollback(ctx)
			t.Fatal("crash_child_prior_transaction_not_committed")
		}
		if err := proof.QueryRow(ctx, `SELECT account.reserved_units,step.maximum_units FROM assistant_usage_accounts AS account
CROSS JOIN assistant_steps AS step WHERE account.account_key='installation' AND step.run_id=$1 AND step.kind='inference' AND step.state='running'`, run.ID).Scan(&held, &maximum); err != nil || held != 100 || maximum != 100 {
			_ = proof.Rollback(ctx)
			t.Fatal("crash_child_reservation_not_visible")
		}
		if proof.Commit(ctx) != nil {
			t.Fatal("crash_child_reservation_proof_failed")
		}
		if phase == "completion_before_commit" {
			if assistantCrashWrite(directory, "dispatch", []byte(assistantCrashDispatchMarker)) != nil {
				t.Fatal("crash_child_dispatch_marker_failed")
			}
			barrier := strconv.FormatInt(assistantCrashBarrier(schema), 10)
			if _, err := store.pool.Exec(ctx, `CREATE FUNCTION assistant_crash_pause() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN PERFORM pg_advisory_xact_lock(`+barrier+`); RETURN NEW; END; $$;
CREATE TRIGGER assistant_crash_pause BEFORE INSERT ON assistant_status_events FOR EACH ROW
WHEN (NEW.state='running') EXECUTE FUNCTION assistant_crash_pause()`); err != nil {
				t.Fatal("crash_child_completion_barrier_failed")
			}
		}
	}
	checkpoint := assistantCrashCheckpoint{Phase: phase, RunID: run.ID, Version: run.Version, Fence: run.Fence, Step: stepNumber}
	encoded, err := json.Marshal(checkpoint)
	if err != nil || assistantCrashWrite(directory, "checkpoint.pending", encoded) != nil {
		t.Fatal("crash_child_checkpoint_failed")
	}
	if stepNumber != 0 {
		// Flush private files first, then use the production minimum lease.
		// Recovery waits for actual server time; no fixture timestamps are changed.
		run, err = store.RenewAssistantLease(ctx, assistantLease(run, time.Now().UTC().Truncate(time.Microsecond)), assistant.MinimumLeaseDuration)
		if err != nil {
			t.Fatal("crash_child_short_owned_lease_failed")
		}
	}
	if os.Rename(filepath.Join(directory, "checkpoint.pending"), filepath.Join(directory, "checkpoint.json")) != nil {
		t.Fatal("crash_child_checkpoint_publish_failed")
	}
	if phase == "completion_before_commit" {
		if _, err := store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, time.Now().UTC().Truncate(time.Microsecond)), Number: stepNumber, Units: 10}); err != nil {
			t.Fatal("crash_child_completion_failed_before_kill")
		}
		t.Fatal("crash_child_completion_unexpectedly_committed")
	}
	<-ctx.Done()
	t.Fatal("crash_child_was_not_killed")
}

func assistantCrashAdmission(actor assistant.Actor, phase string, at time.Time) assistant.Admission {
	a := assistantAdmission(actor, "native_crash_"+phase, at)
	a.Volatile, a.MemoryOwner = true, "crash_original_process"
	return a
}

func assistantCrashSchema(schema string) bool {
	return regexp.MustCompile(`^stackplore_test_[0-9a-f]{16}$`).MatchString(schema)
}

func assistantCrashBarrier(schema string) int64 {
	digest := sha256.Sum256([]byte("stackplore-native-crash-v1\x00" + schema))
	return int64(binary.BigEndian.Uint64(digest[:8]) & ((1 << 63) - 1))
}

func assistantCrashEnvironment(connection, schema, directory, nonce, phase string, at time.Time) []string {
	result := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		for _, allowed := range []string{"SYSTEMROOT", "WINDIR", "SYSTEMDRIVE", "TEMP", "TMP"} {
			if strings.EqualFold(key, allowed) {
				result = append(result, item)
			}
		}
	}
	return append(result, assistantCrashChildKey+"=v1", integrationDatabaseURLKey+"="+connection,
		"STACKPLORE_TEST_ASSISTANT_CRASH_SCHEMA="+schema, "STACKPLORE_TEST_ASSISTANT_CRASH_DIRECTORY="+directory,
		"STACKPLORE_TEST_ASSISTANT_CRASH_NONCE="+nonce, "STACKPLORE_TEST_ASSISTANT_CRASH_PHASE="+phase,
		"STACKPLORE_TEST_ASSISTANT_CRASH_AT="+strconv.FormatInt(at.UnixMicro(), 10))
}

func assistantCrashWrite(directory, name string, contents []byte) error {
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(contents)
	syncErr, closeErr := file.Sync(), file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func assistantCrashRead(directory, name string) ([]byte, error) {
	info, err := os.Lstat(filepath.Join(directory, name))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, assistant.ErrInvalid
	}
	file, err := os.Open(filepath.Join(directory, name))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, 4097))
}

func assistantCrashWaitCheckpoint(t *testing.T, ctx context.Context, directory, phase string, finished <-chan error, reaped *bool) assistantCrashCheckpoint {
	t.Helper()
	wait, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		contents, err := assistantCrashRead(directory, "checkpoint.json")
		if err == nil {
			var checkpoint assistantCrashCheckpoint
			decoder := json.NewDecoder(strings.NewReader(string(contents)))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&checkpoint) != nil || decoder.Decode(new(any)) != io.EOF || checkpoint.Phase != phase || checkpoint.RunID != "native_crash_"+phase || checkpoint.Version < 1 || checkpoint.Fence < 0 || (phase == "admitted" && checkpoint.Step != 0) || (phase != "admitted" && (checkpoint.Step != 1 || checkpoint.Fence < 1)) {
				t.Fatal("crash_parent_checkpoint_invalid")
			}
			return checkpoint
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal("crash_parent_checkpoint_read_failed")
		}
		select {
		case <-finished:
			*reaped = true
			t.Fatal("crash_parent_child_exited_before_checkpoint")
		case <-wait.Done():
			t.Fatal("crash_parent_checkpoint_timeout")
		case <-ticker.C:
		}
	}
}
