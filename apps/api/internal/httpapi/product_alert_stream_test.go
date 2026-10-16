package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/realtime"
)

type durableAlertReaderStub struct {
	position int64
	order    []string
	list     func(context.Context, domain.SourceID, int64, int) (domain.AlertChangePage, error)
}

func (reader *durableAlertReaderStub) ListAlerts(_ context.Context, _ domain.SourceID, page domain.PageRequest) (domain.AlertPage, error) {
	reader.order = append(reader.order, "alerts")
	return domain.AlertPage{Alerts: demo.MustNew().Alerts(), Page: domain.PageInfo{Limit: page.Limit}}, nil
}
func (reader *durableAlertReaderStub) AlertChangePosition(context.Context, domain.SourceID) (int64, error) {
	reader.order = append(reader.order, "position")
	return reader.position, nil
}
func (reader *durableAlertReaderStub) ListAlertChanges(ctx context.Context, source domain.SourceID, after int64, limit int) (domain.AlertChangePage, error) {
	return reader.list(ctx, source, after, limit)
}

func alertChangeFixture(id int64) domain.AlertChange {
	alertID := domain.AlertID("alert_demo_checkout")
	return domain.AlertChange{ID: id, SourceID: demo.SourceID, RuleID: "rule_demo_checkout_health", AlertID: &alertID, Kind: domain.AlertChangeAlert, OccurredAt: demo.ObservationTime()}
}

func TestDurableAlertInitialSnapshotReadsPositionBeforeBoundedPage(t *testing.T) {
	reader := &durableAlertReaderStub{position: 12}
	handler := productHTTP{alertReader: reader}
	frames, position, err := handler.initialAlertFrames(context.Background(), demo.SourceID, reader, 0)
	if err != nil || position != 12 || len(frames) != 1 || frames[0].ID != 12 || frames[0].Event != "alerts.snapshot.v1" || len(reader.order) != 2 || reader.order[0] != "position" || reader.order[1] != "alerts" {
		t.Fatal("snapshot could skip a concurrent committed change")
	}
	var envelope struct {
		Data alertPageResponse `json:"data"`
	}
	if json.Unmarshal(frames[0].Data, &envelope) != nil || len(envelope.Data.Items) != 1 || envelope.Data.Page.Limit != alertStreamPageSize {
		t.Fatal("snapshot omitted bounded page metadata")
	}
}

func TestDurableAlertReplayUsesCommittedIDAndGapRefetches(t *testing.T) {
	reader := &durableAlertReaderStub{position: 18, list: func(_ context.Context, source domain.SourceID, after int64, limit int) (domain.AlertChangePage, error) {
		if source != demo.SourceID || after != 12 || limit != 100 {
			t.Fatal("replay query escaped scope or bound")
		}
		return domain.AlertChangePage{Changes: []domain.AlertChange{alertChangeFixture(15)}, NextID: 15}, nil
	}}
	handler := productHTTP{alertReader: reader}
	frames, position, err := handler.initialAlertFrames(context.Background(), demo.SourceID, reader, 12)
	if err != nil || position != 15 || len(frames) != 1 || frames[0].ID != 15 || frames[0].Event != "alerts.changed.v1" || !bytes.Contains(frames[0].Data, []byte(`"id":"15"`)) || len(reader.order) != 0 {
		t.Fatal("durable replay did not preserve commit cursor")
	}
	reader.list = func(context.Context, domain.SourceID, int64, int) (domain.AlertChangePage, error) {
		return domain.AlertChangePage{Reset: true}, nil
	}
	frames, position, err = handler.initialAlertFrames(context.Background(), demo.SourceID, reader, 12)
	if err != nil || position != 18 || len(frames) != 2 || frames[0].Event != "stream.reset.v1" || frames[1].Event != "alerts.snapshot.v1" {
		t.Fatal("replay gap did not reset and refetch")
	}
}

func TestDurableAlertProducerStopsOnBackpressureAndCancellation(t *testing.T) {
	reader := &durableAlertReaderStub{list: func(_ context.Context, _ domain.SourceID, after int64, _ int) (domain.AlertChangePage, error) {
		return domain.AlertChangePage{Changes: []domain.AlertChange{alertChangeFixture(after + 1)}, NextID: after + 1}, nil
	}}
	queue := make(chan realtime.Frame, alertStreamQueueSize)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if reason := (productHTTP{}).produceAlertChanges(ctx, demo.SourceID, reader, 0, queue, time.Nanosecond); reason != "backpressure" || len(queue) != alertStreamQueueSize {
		t.Fatal("slow client did not stop at queue cap")
	}
	reader.list = func(ctx context.Context, _ domain.SourceID, _ int64, _ int) (domain.AlertChangePage, error) {
		<-ctx.Done()
		return domain.AlertChangePage{}, ctx.Err()
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	started := time.Now()
	_ = (productHTTP{}).produceAlertChanges(cancelled, demo.SourceID, reader, 0, queue, time.Millisecond)
	if time.Since(started) > time.Second {
		t.Fatal("cancelled read did not stop promptly")
	}
}

func TestDurableAlertFrameRejectsWrongScopeAndOrder(t *testing.T) {
	cases := []domain.AlertChangePage{
		{Changes: []domain.AlertChange{alertChangeFixture(10)}, NextID: 10},
		{Changes: []domain.AlertChange{alertChangeFixture(12), alertChangeFixture(11)}, NextID: 11},
		{Changes: []domain.AlertChange{alertChangeFixture(12)}, NextID: 13},
		{Changes: []domain.AlertChange{{ID: 12, SourceID: "source_other", RuleID: "rule_other", Kind: domain.AlertChangeAlert, OccurredAt: demo.ObservationTime()}}, NextID: 12},
	}
	for _, page := range cases {
		if _, err := alertChangesFrame(demo.SourceID, 10, page); err == nil {
			t.Fatal("invalid change metadata crossed stream boundary")
		}
	}
}

func TestDurableAlertFrameRejectsInvalidTimeOrMissingTarget(t *testing.T) {
	for _, at := range []time.Time{time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond), demo.ObservationTime().Add(time.Nanosecond), demo.ObservationTime().In(time.FixedZone("local", 3600))} {
		change := alertChangeFixture(12)
		change.OccurredAt = at
		if _, err := alertChangesFrame(demo.SourceID, 10, domain.AlertChangePage{Changes: []domain.AlertChange{change}, NextID: 12}); err == nil {
			t.Fatal("invalid change timestamp crossed stream boundary")
		}
	}
	change := alertChangeFixture(12)
	change.AlertID = nil
	if _, err := alertChangesFrame(demo.SourceID, 10, domain.AlertChangePage{Changes: []domain.AlertChange{change}, NextID: 12}); err == nil {
		t.Fatal("alert change without episode crossed boundary")
	}
}
