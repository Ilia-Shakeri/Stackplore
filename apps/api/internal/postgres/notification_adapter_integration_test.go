package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func seedRealDeliveryNotification(t *testing.T, fixture observationFixture) {
	t.Helper()
	if _, err := fixture.store.pool.Exec(fixture.ctx, insertNotificationStatement, "notification_adapter_real", nil, nil, nil, "notification.alert_firing", "error", "alert.condition_matched", fixture.at.UnixMicro(), 0); err != nil {
		t.Fatal("seed real delivery fixture")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, insertNotificationStatement, "notification_adapter_demo", string(fixture.source.Source.ID), nil, nil, "notification.alert_firing", "error", "alert.condition_matched", fixture.at.UnixMicro(), 1); err != nil {
		t.Fatal("seed synthetic delivery fixture")
	}
}

func TestNotificationAdapterMigrationPreservesCursorAndCancelsSyntheticQueue(t *testing.T) {
	store, options, admin := openIntegrationStore(t)
	if err := store.Close(); err != nil {
		t.Fatal("close adapter upgrade fixture")
	}
	ctx := context.Background()
	identifier := pgx.Identifier{options.Schema}.Sanitize()
	if _, err := admin.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE; CREATE SCHEMA "+identifier); err != nil {
		t.Fatal("reset isolated adapter upgrade schema")
	}
	files := fstest.MapFS{}
	entries, err := fs.ReadDir(postgresmigrations.FS, ".")
	if err != nil {
		t.Fatal("read adapter migration inventory")
	}
	for _, entry := range entries {
		version, valid := migrationVersion(entry.Name())
		if !valid || version > 9 {
			continue
		}
		body, err := fs.ReadFile(postgresmigrations.FS, entry.Name())
		if err != nil {
			t.Fatal("read earlier migration bytes")
		}
		files[entry.Name()] = &fstest.MapFile{Data: body, Mode: 0444}
	}
	old, err := Open(ctx, options, files)
	if err != nil {
		t.Fatal("open nine-version adapter fixture")
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	source := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_upgrade_demo", Provider: domain.ProviderDemo, Name: "Upgrade fixture", Synthetic: true}, ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}
	if err := old.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{source}); err != nil {
		t.Fatal("seed adapter upgrade source")
	}
	item := domain.Notification{ID: "notification_upgrade_demo", SourceID: &source.Source.ID, Kind: "notification.alert_firing", Severity: domain.SeverityWarning, SummaryCode: "alert.condition_matched", CreatedAt: at, Synthetic: true}
	if _, err := old.pool.Exec(ctx, insertNotificationStatement, string(item.ID), string(source.Source.ID), nil, nil, string(item.Kind), string(item.Severity), string(item.SummaryCode), at.UnixMicro(), 1); err != nil {
		t.Fatal("seed earlier synthetic notice")
	}
	payload, _, digest, err := renderWebhookDelivery("destination_upgrade", item)
	if err != nil {
		t.Fatal("render earlier synthetic payload")
	}
	for _, id := range []string{"delivery_upgrade_pending", "delivery_upgrade_inflight"} {
		if _, err := old.pool.Exec(ctx, insertWebhookDelivery, id, string(item.ID), "alert", nil, "destination_upgrade", payload, digest[:], 3, at.UnixMicro(), "webhook"); err != nil {
			t.Fatal("seed earlier delivery")
		}
	}
	if _, err := old.pool.Exec(ctx, `UPDATE notification_deliveries SET state='in_flight',attempt_count=1,next_attempt_at_us=NULL,lease_owner='worker_upgrade',lease_expires_at_us=$1 WHERE id='delivery_upgrade_inflight'`, at.Add(time.Minute).UnixMicro()); err != nil {
		t.Fatal("seed earlier active lease")
	}
	if _, err := old.pool.Exec(ctx, `INSERT INTO notification_delivery_attempts(delivery_id,attempt_number,started_at_us) VALUES('delivery_upgrade_inflight',1,$1)`, at.UnixMicro()); err != nil {
		t.Fatal("seed earlier active attempt")
	}
	if _, err := old.pool.Exec(ctx, `INSERT INTO notification_delivery_cursors(destination_reference_id,last_notification_sequence,created_at_us,updated_at_us) VALUES('destination_upgrade',1,$1,$1)`, at.UnixMicro()); err != nil {
		t.Fatal("seed earlier outbox cursor")
	}
	if err := old.Close(); err != nil {
		t.Fatal("close earlier adapter fixture")
	}
	upgraded, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("upgrade adapter fixture")
	}
	defer upgraded.Close()
	var canceled, closed, cursors int
	if err := upgraded.pool.QueryRow(ctx, `SELECT count(*) FROM notification_deliveries WHERE state='canceled' AND lease_owner IS NULL AND next_attempt_at_us IS NULL`).Scan(&canceled); err != nil || canceled != 2 {
		t.Fatal("upgrade kept synthetic deliveries runnable")
	}
	if err := upgraded.pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_attempts WHERE outcome='canceled' AND finished_at_us IS NOT NULL`).Scan(&closed); err != nil || closed != 1 {
		t.Fatal("upgrade kept synthetic attempt open")
	}
	if err := upgraded.pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_cursors WHERE adapter_kind='webhook' AND destination_reference_id='destination_upgrade' AND last_notification_sequence=1`).Scan(&cursors); err != nil || cursors != 1 {
		t.Fatal("upgrade lost existing webhook cursor")
	}
}

func TestNotificationAdapterLanesPersistAndFenceExpiredWorkers(t *testing.T) {
	fixture := newObservationFixture(t)
	seedRealDeliveryNotification(t, fixture)
	store, ctx, at := fixture.store, fixture.ctx, fixture.at
	for _, adapter := range []notification.DeliveryAdapter{notification.DeliveryAdapterWebhook, notification.DeliveryAdapterMatrix} {
		created, err := store.EnqueueDeliveries(ctx, adapter, "destination_shared", 3, at, 32)
		if err != nil || created != 1 {
			t.Fatal("adapter cursor did not queue exactly one real notice")
		}
		created, err = store.EnqueueDeliveries(ctx, adapter, "destination_shared", 3, at, 32)
		if err != nil || created != 0 {
			t.Fatal("adapter cursor did not retain stable enqueue position")
		}
	}
	matrix, err := store.ClaimDeliveries(ctx, notification.DeliveryAdapterMatrix, "destination_shared", "worker_matrix", at, time.Second, 1)
	if err != nil || len(matrix) != 1 || !strings.Contains(string(matrix[0].Payload), `"msgtype":"m.notice"`) {
		t.Fatal("Matrix lane did not claim its plain notice")
	}
	webhook, err := store.ClaimDeliveries(ctx, notification.DeliveryAdapterWebhook, "destination_shared", "worker_webhook", at, time.Minute, 1)
	if err != nil || len(webhook) != 1 || matrix[0].ID == webhook[0].ID || strings.Contains(string(webhook[0].Payload), `"msgtype"`) {
		t.Fatal("webhook and Matrix lanes overlapped")
	}
	status := http.StatusOK
	completion := notification.DeliveryCompletion{DeliveryID: matrix[0].ID, AttemptNumber: 1, LeaseOwner: "worker_matrix", FinishedAt: at, Outcome: notification.DeliverySucceeded, ResponseStatus: &status}
	if err := store.CompleteDelivery(ctx, notification.DeliveryAdapterWebhook, "destination_shared", completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("wrong adapter completed another lane")
	}
	if err := store.CompleteDelivery(ctx, notification.DeliveryAdapterMatrix, "destination_other", completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("wrong reference completed another lane")
	}
	completion.FinishedAt = at.Add(time.Second)
	if err := store.CompleteDelivery(ctx, notification.DeliveryAdapterMatrix, "destination_shared", completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("expired lease completed delivery")
	}
	if err := store.Close(); err != nil {
		t.Fatal("close claimed delivery fixture")
	}
	reopened, err := Open(ctx, fixture.options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("reopen claimed delivery fixture")
	}
	defer reopened.Close()
	recovered, err := reopened.ClaimDeliveries(ctx, notification.DeliveryAdapterMatrix, "destination_shared", "worker_recovered", at.Add(time.Second), time.Minute, 1)
	if err != nil || len(recovered) != 1 || recovered[0].ID != matrix[0].ID || recovered[0].AttemptNumber != 2 || string(recovered[0].Payload) != string(matrix[0].Payload) {
		t.Fatal("restart did not recover stable Matrix delivery")
	}
	completion.FinishedAt = at
	if err := reopened.CompleteDelivery(ctx, notification.DeliveryAdapterMatrix, "destination_shared", completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("old owner completed recovered delivery")
	}
	completion.LeaseOwner, completion.AttemptNumber, completion.FinishedAt = "worker_recovered", 2, at.Add(time.Second)
	if err := reopened.CompleteDelivery(ctx, notification.DeliveryAdapterMatrix, "destination_shared", completion); err != nil {
		t.Fatal("recovered owner failed to complete delivery")
	}
	var uncertain, synthetic int
	if err := reopened.pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_attempts WHERE delivery_id=$1 AND attempt_number=1 AND outcome='uncertain' AND error_code='notification.lease_expired'`, matrix[0].ID).Scan(&uncertain); err != nil || uncertain != 1 {
		t.Fatal("expired attempt lacks durable uncertain result")
	}
	if err := reopened.pool.QueryRow(ctx, `SELECT count(*) FROM notification_deliveries WHERE notification_id='notification_adapter_demo'`).Scan(&synthetic); err != nil || synthetic != 0 {
		t.Fatal("synthetic notice escaped to delivery lanes")
	}
}

func TestNotificationCompletionRejectsPastClockAndExhaustedRetry(t *testing.T) {
	fixture := newObservationFixture(t)
	seedRealDeliveryNotification(t, fixture)
	if created, err := fixture.store.EnqueueDeliveries(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_fixture", 1, fixture.at, 32); err != nil || created != 1 {
		t.Fatal("queue completion fixture")
	}
	tasks, err := fixture.store.ClaimDeliveries(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_fixture", "worker_fixture", fixture.at, time.Minute, 1)
	if err != nil || len(tasks) != 1 {
		t.Fatal("claim completion fixture")
	}
	status := http.StatusServiceUnavailable
	retryAt := fixture.at.Add(time.Second)
	completion := notification.DeliveryCompletion{DeliveryID: tasks[0].ID, AttemptNumber: 1, LeaseOwner: "worker_fixture", FinishedAt: fixture.at, Outcome: notification.DeliveryFailed, ResponseStatus: &status, ErrorCode: "notification.remote_rejected", RetryAt: &retryAt}
	if err := fixture.store.CompleteDelivery(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_fixture", completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("exhausted retry left unrunnable pending delivery")
	}
	completion.RetryAt = nil
	completion.FinishedAt = fixture.at.Add(-time.Microsecond)
	if err := fixture.store.CompleteDelivery(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_fixture", completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("completion predates its active attempt")
	}
	completion.FinishedAt = fixture.at
	if err := fixture.store.CompleteDelivery(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_fixture", completion); err != nil {
		t.Fatal("terminal failed completion rejected")
	}
	if tasks, err := fixture.store.ClaimDeliveries(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_fixture", "worker_fixture", retryAt, time.Minute, 1); err != nil || len(tasks) != 0 {
		t.Fatal("exhausted delivery ran again")
	}
}

type deliveryFixtureResolver struct{}

func (deliveryFixtureResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
}

type deliveryFixtureDialer struct{ address string }

func (dialer deliveryFixtureDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if address != "8.8.8.8:443" {
		return nil, errors.New("fixture pinned address differs")
	}
	return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, network, dialer.address)
}

func TestPersistedAdapterRetriesReachDisposableHTTPSReceiver(t *testing.T) {
	fixture := newObservationFixture(t)
	seedRealDeliveryNotification(t, fixture)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("generate disposable delivery credentials")
	}
	defer clear(key)
	token := []byte(base64.RawURLEncoding.EncodeToString(key))
	defer clear(token)
	type received struct {
		id       string
		digest   [32]byte
		requests int
	}
	seen := make(map[string]received)
	var mu sync.Mutex
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(io.LimitReader(request.Body, 65537))
		if err != nil || len(body) > 65536 {
			t.Error("disposable receiver read bound failed")
			response.WriteHeader(400)
			return
		}
		adapter, id := "webhook", request.Header.Get("Idempotency-Key")
		if request.Method == http.MethodPut {
			adapter = "matrix"
			id = request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:]
			if !strings.HasPrefix(request.URL.EscapedPath(), "/_matrix/client/v3/rooms/%21fixture/send/m.room.message/") || request.Header.Get("Authorization") != "Bearer "+string(token) || !strings.Contains(string(body), `"msgtype":"m.notice"`) {
				t.Error("Matrix receiver transaction, auth, or plain payload differs")
			}
		} else {
			mac := hmac.New(sha256.New, key)
			_, _ = mac.Write([]byte(id))
			_, _ = mac.Write([]byte{0})
			_, _ = mac.Write(body)
			if request.Method != http.MethodPost || id == "" || request.Header.Get("X-Stackplore-Delivery-ID") != id || !hmac.Equal([]byte(request.Header.Get("X-Stackplore-Signature")), []byte("sha256="+hex.EncodeToString(mac.Sum(nil)))) {
				t.Error("webhook receiver signature or stable key differs")
			}
		}
		mu.Lock()
		prior := seen[adapter]
		digest := sha256.Sum256(body)
		if prior.requests != 0 && (prior.id != id || prior.digest != digest) {
			t.Error("retry changed persisted delivery identity or canonical bytes")
		}
		prior.id, prior.digest, prior.requests = id, digest, prior.requests+1
		seen[adapter] = prior
		mu.Unlock()
		if prior.requests == 1 {
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if adapter == "matrix" {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"event_id":"$fixture"}`))
			return
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	options := notification.WebhookSenderOptions{Resolver: deliveryFixtureResolver{}, Dialer: deliveryFixtureDialer{address: server.Listener.Addr().String()}, Timeout: time.Second, MaximumResponseBytes: 1024, RootCAs: pool}
	webhookSender, err := notification.NewWebhookSender(options)
	if err != nil {
		t.Fatal("create webhook fixture sender")
	}
	matrixSender, err := notification.NewMatrixSender(options)
	if err != nil {
		t.Fatal("create Matrix fixture sender")
	}
	webhookTargets, err := notification.NewStaticWebhookTargetResolver("destination_webhook", "https://example.com/fixture", key)
	if err != nil {
		t.Fatal("create webhook fixture target")
	}
	defer webhookTargets.Close()
	matrixTargets, err := notification.NewStaticMatrixTargetResolver("destination_matrix", "https://example.com", "!fixture", token)
	if err != nil {
		t.Fatal("create Matrix fixture target")
	}
	defer matrixTargets.Close()
	at := fixture.at.Add(time.Minute)
	for _, adapter := range []notification.DeliveryAdapter{notification.DeliveryAdapterWebhook, notification.DeliveryAdapterMatrix} {
		reference := "destination_" + string(adapter)
		workerOptions := notification.DeliveryWorkerOptions{AdapterRepository: fixture.store, Adapter: adapter, DestinationReferenceID: reference, LeaseOwner: "worker_" + string(adapter), Now: func() time.Time { return at }, PollInterval: time.Second, LeaseDuration: 5 * time.Second, BatchSize: 1, MaximumAttempts: 3, RetrySchedule: []time.Duration{time.Second}, OnError: func(error) { t.Error("fixture worker failed to persist result") }}
		if adapter == notification.DeliveryAdapterWebhook {
			workerOptions.Targets, workerOptions.Sender = webhookTargets, webhookSender
		} else {
			workerOptions.Attempt = notification.MatrixAttempt(matrixTargets, matrixSender)
		}
		worker, err := notification.NewDeliveryWorker(workerOptions)
		if err != nil {
			t.Fatal("create adapter fixture worker")
		}
		if err := worker.RunOnce(fixture.ctx); err != nil {
			t.Fatal("first persisted attempt failed")
		}
		at = at.Add(time.Second)
		if err := worker.RunOnce(fixture.ctx); err != nil {
			t.Fatal("persisted retry failed")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen["webhook"].requests != 2 || seen["matrix"].requests != 2 {
		t.Fatal("receiver did not observe exactly two stable attempts per adapter")
	}
	var delivered, attempts int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM notification_deliveries WHERE state='succeeded' AND attempt_count=2`).Scan(&delivered); err != nil || delivered != 2 {
		t.Fatal("successful adapter retries not durable")
	}
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM notification_delivery_attempts WHERE finished_at_us IS NOT NULL`).Scan(&attempts); err != nil || attempts != 4 {
		t.Fatal("attempt outcomes not durable")
	}
}
